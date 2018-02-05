package network

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	sarama "github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/services/errors"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
	"github.com/stephane-martin/skewer/utils/queue/tcp"
)

var connCounter *prometheus.CounterVec
var ackCounter *prometheus.CounterVec
var messageFilterCounter *prometheus.CounterVec

func initDirectRelpRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()

		// as a RELP service
		relpAnswersCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_relp_answers_total",
				Help: "number of RSP answers sent back to the RELP client",
			},
			[]string{"status", "client"},
		)

		relpProtocolErrorsCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_relp_protocol_errors_total",
				Help: "Number of RELP protocol errors",
			},
			[]string{"client"},
		)

		// as a "directrelp destination"
		ackCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_dest_ack_total",
				Help: "number of message acknowledgments",
			},
			[]string{"dest", "status"},
		)

		connCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_dest_conn_total",
				Help: "number of connections to remote service",
			},
			[]string{"dest", "status"},
		)

		messageFilterCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_message_filtering_total",
				Help: "number of filtered messages by status",
			},
			[]string{"status", "client", "destination"},
		)

		base.Registry.MustRegister(relpAnswersCounter, relpProtocolErrorsCounter, ackCounter, connCounter, messageFilterCounter)
	})
}

type DirectRelpService struct {
	impl           *DirectRelpServiceImpl
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
	QueueSize      uint64
	logger         log15.Logger
	reporter       base.Reporter
	b              binder.Client
	sc             []conf.DirectRELPSourceConfig
	pc             []conf.ParserConfig
	kc             conf.KafkaDestConfig
	wg             sync.WaitGroup
	confined       bool
}

func NewDirectRelpService(env *base.ProviderEnv) (base.Provider, error) {
	initDirectRelpRegistry()
	s := DirectRelpService{
		b:        env.Binder,
		logger:   env.Logger,
		reporter: env.Reporter,
		confined: env.Confined,
	}
	s.impl = NewDirectRelpServiceImpl(env.Confined, env.Reporter, env.Binder, env.Logger)
	return &s, nil
}

func (s *DirectRelpService) Type() base.Types {
	return base.DirectRELP
}

func (s *DirectRelpService) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *DirectRelpService) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *DirectRelpService) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *DirectRelpService) Start() (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	s.impl = NewDirectRelpServiceImpl(s.confined, s.reporter, s.b, s.logger)
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			state := <-s.impl.StatusChan
			switch state {
			case FinalStopped:
				_ = s.reporter.Report([]model.ListenerInfo{})
				return

			case Stopped:
				s.impl.SetConf(s.sc, s.pc, s.kc, s.QueueSize)
				infos, err := s.impl.Start()
				if err == nil {
					err = s.reporter.Report(infos)
					if err != nil {
						s.impl.Logger.Error("Failed to report infos. Fatal error.", "error", err)
						s.dofatal()
					}
				} else {
					s.impl.Logger.Warn("The DirectRELP service has failed to start", "error", err)
					err = s.reporter.Report([]model.ListenerInfo{})
					if err != nil {
						s.impl.Logger.Error("Failed to report infos. Fatal error.", "error", err)
						s.dofatal()
					} else {
						s.impl.StopAndWait()
					}
				}

			case Waiting:
				go func() {
					time.Sleep(time.Duration(30) * time.Second)
					s.impl.EndWait()
				}()

			case Started:
			}
		}
	}()

	s.impl.StatusChan <- Stopped // trigger the RELP service to start
	return
}

func (s *DirectRelpService) Shutdown() {
	s.Stop()
}

func (s *DirectRelpService) Stop() {
	s.impl.FinalStop()
	s.wg.Wait()
}

func (s *DirectRelpService) SetConf(c conf.BaseConfig) {
	s.sc = c.DirectRELPSource
	s.pc = c.Parsers
	s.kc = *c.KafkaDest
	s.QueueSize = c.Main.InputQueueSize
}

type DirectRelpServiceImpl struct {
	StreamingService
	RelpConfigs         []conf.DirectRELPSourceConfig
	kafkaConf           conf.KafkaDestConfig
	status              RelpServerStatus
	StatusChan          chan RelpServerStatus
	producer            sarama.AsyncProducer
	reporter            base.Reporter
	rawMessagesQueue    *tcp.Ring
	parsedMessagesQueue *queue.MessageQueue
	parsewg             sync.WaitGroup
	configs             map[utils.MyULID]conf.DirectRELPSourceConfig
	forwarder           *ackForwarder
}

func NewDirectRelpServiceImpl(confined bool, reporter base.Reporter, b binder.Client, logger log15.Logger) *DirectRelpServiceImpl {
	s := DirectRelpServiceImpl{
		status:    Stopped,
		reporter:  reporter,
		configs:   map[utils.MyULID]conf.DirectRELPSourceConfig{},
		forwarder: newAckForwarder(),
	}
	s.StreamingService.init()
	s.StreamingService.BaseService.Logger = logger.New("class", "DirectRELPService")
	s.StreamingService.BaseService.Binder = b
	s.StreamingService.handler = DirectRelpHandler{Server: &s}
	s.StreamingService.confined = confined
	s.StatusChan = make(chan RelpServerStatus, 10)
	return &s
}

func (s *DirectRelpServiceImpl) Start() ([]model.ListenerInfo, error) {
	s.LockStatus()
	defer s.UnlockStatus()
	if s.status == FinalStopped {
		return nil, errors.ServerDefinitelyStopped
	}
	if s.status != Stopped && s.status != Waiting {
		return nil, errors.ServerNotStopped
	}

	infos := s.initTCPListeners()
	if len(infos) == 0 {
		s.Logger.Info("DirectRELP service not started: no listener")
		return infos, nil
	}

	var err error
	s.producer, err = s.kafkaConf.GetAsyncProducer(s.confined)
	if err != nil {
		connCounter.WithLabelValues("directkafka", "fail").Inc()
		s.resetTCPListeners()
		return nil, err
	}
	connCounter.WithLabelValues("directkafka", "success").Inc()

	s.Logger.Info("Listening on DirectRELP", "nb_services", len(infos))

	s.parsedMessagesQueue = queue.NewMessageQueue()
	s.rawMessagesQueue = tcp.NewRing(s.QueueSize)
	s.configs = map[utils.MyULID]conf.DirectRELPSourceConfig{}

	for _, l := range s.UnixListeners {
		s.configs[l.Conf.ConfID] = conf.DirectRELPSourceConfig(l.Conf)
	}
	for _, l := range s.TcpListeners {
		s.configs[l.Conf.ConfID] = conf.DirectRELPSourceConfig(l.Conf)
	}

	s.wg.Add(1)
	go s.push2kafka()
	s.wg.Add(1)
	go s.handleKafkaResponses()

	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.parsewg.Add(1)
		go s.parse()
	}

	s.status = Started
	s.StatusChan <- Started

	s.Listen()
	return infos, nil
}

func (s *DirectRelpServiceImpl) Stop() {
	s.LockStatus()
	s.doStop(false, false)
	s.UnlockStatus()
}

func (s *DirectRelpServiceImpl) FinalStop() {
	s.LockStatus()
	s.doStop(true, false)
	s.UnlockStatus()
}

func (s *DirectRelpServiceImpl) StopAndWait() {
	s.LockStatus()
	s.doStop(false, true)
	s.UnlockStatus()
}

func (s *DirectRelpServiceImpl) EndWait() {
	s.LockStatus()
	if s.status != Waiting {
		s.UnlockStatus()
		return
	}
	s.status = Stopped
	s.StatusChan <- Stopped
	s.UnlockStatus()
}

func (s *DirectRelpServiceImpl) doStop(final bool, wait bool) {
	if final && (s.status == Waiting || s.status == Stopped || s.status == FinalStopped) {
		if s.status != FinalStopped {
			s.status = FinalStopped
			s.StatusChan <- FinalStopped
			close(s.StatusChan)
		}
		return
	}

	if s.status == Stopped || s.status == FinalStopped || s.status == Waiting {
		if s.status == Stopped && wait {
			s.status = Waiting
			s.StatusChan <- Waiting
		}
		return
	}

	s.resetTCPListeners() // makes the listeners stop
	// no more message will arrive in rawMessagesQueue
	if s.rawMessagesQueue != nil {
		s.rawMessagesQueue.Dispose()
	}
	// the parsers consume the rest of rawMessagesQueue, then they stop
	s.parsewg.Wait() // wait that the parsers have stopped
	if s.parsedMessagesQueue != nil {
		s.parsedMessagesQueue.Dispose()
	}

	// after the parsers have stopped, we can close the queues
	s.forwarder.RemoveAll()
	// wait that all goroutines have ended
	s.wg.Wait()

	if final {
		s.status = FinalStopped
		s.StatusChan <- FinalStopped
		close(s.StatusChan)
	} else if wait {
		s.status = Waiting
		s.StatusChan <- Waiting
	} else {
		s.status = Stopped
		s.StatusChan <- Stopped
	}
}

func (s *DirectRelpServiceImpl) SetConf(sc []conf.DirectRELPSourceConfig, pc []conf.ParserConfig, kc conf.KafkaDestConfig, queueSize uint64) {
	tcpConfigs := []conf.TCPSourceConfig{}
	for _, c := range sc {
		tcpConfigs = append(tcpConfigs, conf.TCPSourceConfig(c))
	}
	s.StreamingService.SetConf(tcpConfigs, pc, queueSize, 132000)
	s.kafkaConf = kc
	s.BaseService.Pool = &sync.Pool{New: func() interface{} {
		return &model.RawTcpMessage{Message: make([]byte, 132000)}
	}}
}

func makeDRELPLogger(logger log15.Logger, raw *model.RawTcpMessage) log15.Logger {
	return logger.New(
		"protocol", "directrelp",
		"client", raw.Client,
		"local_port", raw.LocalPort,
		"unix_socket_path", raw.UnixSocketPath,
		"format", raw.Format,
		"txnr", raw.Txnr,
	)
}

func (s *DirectRelpServiceImpl) parseOne(raw *model.RawTcpMessage, e *base.ParsersEnv) {

	parser, err := e.GetParser(raw.Format)
	if err != nil || parser == nil {
		s.forwarder.ForwardFail(raw.ConnID, raw.Txnr)
		makeDRELPLogger(s.Logger, raw).Crit("Unknown parser")
		return
	}
	decoder := utils.SelectDecoder(raw.Encoding)
	syslogMsg, err := parser(raw.Message, decoder)
	if err != nil {
		makeDRELPLogger(s.Logger, raw).Warn("Parsing error", "error", err)
		s.forwarder.ForwardFail(raw.ConnID, raw.Txnr)
		base.ParsingErrorCounter.WithLabelValues("directrelp", raw.Client, raw.Format).Inc()
		return
	}
	if syslogMsg == nil {
		s.forwarder.ForwardSucc(raw.ConnID, raw.Txnr)
		return
	}

	if raw.Client != "" {
		syslogMsg.SetProperty("skewer", "client", raw.Client)
	}
	if raw.LocalPort != 0 {
		syslogMsg.SetProperty("skewer", "localport", strconv.FormatInt(int64(raw.LocalPort), 10))
	}
	if raw.UnixSocketPath != "" {
		syslogMsg.SetProperty("skewer", "socketpath", raw.UnixSocketPath)
	}

	full := model.FullFactoryFrom(syslogMsg)
	full.Txnr = raw.Txnr
	full.ConfId = raw.ConfID
	full.ConnId = raw.ConnID
	_ = s.parsedMessagesQueue.Put(full)
}

func (s *DirectRelpServiceImpl) parse() {
	defer s.parsewg.Done()

	var raw *model.RawTcpMessage
	var err error
	e := base.NewParsersEnv(s.ParserConfigs, s.Logger)

	for {
		raw, err = s.rawMessagesQueue.Get()
		if err != nil {
			return
		}
		if raw == nil {
			s.Logger.Error("rawMessagesQueue returns nil, should not happen!")
			return
		}
		s.parseOne(raw, e)
		s.Pool.Put(raw)
	}
}

func (s *DirectRelpServiceImpl) handleKafkaResponses() {
	var succ *sarama.ProducerMessage
	var fail *sarama.ProducerError
	var more, fatal bool
	kafkaSuccChan := s.producer.Successes()
	kafkaFailChan := s.producer.Errors()
	for {
		if kafkaSuccChan == nil && kafkaFailChan == nil {
			return
		}
		select {
		case succ, more = <-kafkaSuccChan:
			if more {
				metad := succ.Metadata.(meta)
				s.forwarder.ForwardSucc(metad.ConnID, metad.Txnr)
			} else {
				kafkaSuccChan = nil
			}
		case fail, more = <-kafkaFailChan:
			if more {
				metad := fail.Msg.Metadata.(meta)
				s.forwarder.ForwardFail(metad.ConnID, metad.Txnr)
				s.Logger.Info("NACK from Kafka", "error", fail.Error(), "txnr", metad.Txnr, "topic", fail.Msg.Topic)
				fatal = model.IsFatalKafkaError(fail.Err)
			} else {
				kafkaFailChan = nil
			}

		}

		if fatal {
			s.StopAndWait()
			return
		}

	}

}

func (s *DirectRelpServiceImpl) handleResponses(conn net.Conn, connID utils.MyULID, client string, logger log15.Logger) {
	defer func() {
		s.wg.Done()
	}()

	successes := map[int32]bool{}
	failures := map[int32]bool{}
	var err error

	writeSuccess := func(txnr int32) (err error) {
		_, err = fmt.Fprintf(conn, "%d rsp 6 200 OK\n", txnr)
		return err
	}

	writeFailure := func(txnr int32) (err error) {
		_, err = fmt.Fprintf(conn, "%d rsp 6 500 KO\n", txnr)
		return err
	}

	for s.forwarder.Wait(connID) {
		currentTxnr := s.forwarder.GetSucc(connID)
		if currentTxnr != -1 {
			//logger.Debug("New success to report to client", "txnr", currentTxnr)
			successes[currentTxnr] = true
		}

		currentTxnr = s.forwarder.GetFail(connID)
		if currentTxnr != -1 {
			//logger.Debug("New failure to report to client", "txnr", currentTxnr)
			failures[currentTxnr] = true
		}

		// rsyslog expects the ACK/txnr correctly and monotonously ordered
		// so we need a bit of cooking to ensure that
	Cooking:
		for {
			next := s.forwarder.NextToCommit(connID)
			if next == -1 {
				break Cooking
			}
			//logger.Debug("Next to commit", "connid", connID, "txnr", next)
			if successes[next] {
				err = writeSuccess(next)
				if err == nil {
					//logger.Debug("ACK to client", "connid", connID, "tnxr", next)
					delete(successes, next)
					relpAnswersCounter.WithLabelValues("200", client).Inc()
					ackCounter.WithLabelValues("directrelp", "ack").Inc()
				}
			} else if failures[next] {
				err = writeFailure(next)
				if err == nil {
					//logger.Debug("NACK to client", "connid", connID, "txnr", next)
					delete(failures, next)
					relpAnswersCounter.WithLabelValues("500", client).Inc()
					ackCounter.WithLabelValues("directrelp", "nack").Inc()
				}
			} else {
				break Cooking
			}

			if err == nil {
				s.forwarder.Commit(connID)
			} else if err == io.EOF {
				// client is gone
				return
			} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				logger.Info("Timeout error writing RELP response to client", "error", err)
			} else {
				logger.Warn("Unexpected error writing RELP response to client", "error", err)
				return
			}
		}
	}
}

func (s *DirectRelpServiceImpl) push2kafka() {
	defer func() {
		s.producer.AsyncClose()
		s.wg.Done()
	}()
	envs := map[utils.MyULID]*javascript.Environment{}
	var message *model.FullMessage
	var err error

	for s.parsedMessagesQueue.Wait(0) {
		message, err = s.parsedMessagesQueue.Get()
		if message == nil || err != nil {
			// should not happen
			s.Logger.Error("Fatal error getting messages from the parsed messages queue", "error", err)
			s.StopAndWait()
			return
		}
		s.pushOne(message, &envs)
	}
}

func (s *DirectRelpServiceImpl) pushOne(message *model.FullMessage, envs *map[utils.MyULID]*javascript.Environment) {
	defer model.FullFree(message)
	var err error

	e, haveEnv := (*envs)[message.ConfId]
	if !haveEnv {
		config, haveConfig := s.configs[message.ConfId]
		if !haveConfig {
			s.Logger.Warn("Could not find the configuration for a message", "confId", message.ConfId, "txnr", message.Txnr)
			return
		}
		(*envs)[message.ConfId] = javascript.NewFilterEnvironment(
			config.FilterFunc,
			config.TopicFunc,
			config.TopicTmpl,
			config.PartitionFunc,
			config.PartitionTmpl,
			config.PartitionNumberFunc,
			s.Logger,
		)
		e = (*envs)[message.ConfId]
	}

	topic, errs := e.Topic(message.Fields)
	for _, err = range errs {
		s.Logger.Info("Error calculating topic", "error", err, "txnr", message.Txnr)
	}
	if len(topic) == 0 {
		s.Logger.Warn("Topic or PartitionKey could not be calculated", "txnr", message.Txnr)
		s.forwarder.ForwardFail(message.ConnId, message.Txnr)
		return
	}
	partitionKey, errs := e.PartitionKey(message.Fields)
	for _, err = range errs {
		s.Logger.Info("Error calculating the partition key", "error", err, "txnr", message.Txnr)
	}
	partitionNumber, errs := e.PartitionNumber(message.Fields)
	for _, err = range errs {
		s.Logger.Info("Error calculating the partition number", "error", err, "txnr", message.Txnr)
	}

	filterResult, err := e.FilterMessage(message.Fields)
	if err != nil {
		s.Logger.Warn("Error happened filtering message", "error", err)
		return
	}

	switch filterResult {
	case javascript.DROPPED:
		s.forwarder.ForwardFail(message.ConnId, message.Txnr)
		messageFilterCounter.WithLabelValues("dropped", message.Fields.GetProperty("skewer", "client"), "directkafka").Inc()
		return
	case javascript.REJECTED:
		s.forwarder.ForwardFail(message.ConnId, message.Txnr)
		messageFilterCounter.WithLabelValues("rejected", message.Fields.GetProperty("skewer", "client"), "directkafka").Inc()
		return
	case javascript.PASS:
		messageFilterCounter.WithLabelValues("passing", message.Fields.GetProperty("skewer", "client"), "directkafka").Inc()
	default:
		s.forwarder.ForwardFail(message.ConnId, message.Txnr)
		messageFilterCounter.WithLabelValues("unknown", message.Fields.GetProperty("skewer", "client"), "directkafka").Inc()
		s.Logger.Warn("Error happened processing message", "txnr", message.Txnr, "error", err)
		return
	}

	serialized, err := message.Fields.RegularJson()

	if err != nil {
		s.Logger.Warn("Error generating Kafka message", "error", err, "txnr", message.Txnr)
		s.forwarder.ForwardFail(message.ConnId, message.Txnr)
		return
	}

	kafkaMsg := &sarama.ProducerMessage{
		Key:       sarama.StringEncoder(partitionKey),
		Partition: partitionNumber,
		Value:     sarama.ByteEncoder(serialized),
		Topic:     topic,
		Timestamp: message.Fields.GetTimeReported(),
		Metadata:  meta{Txnr: message.Txnr, ConnID: message.ConnId},
	}

	s.producer.Input() <- kafkaMsg
}

type DirectRelpHandler struct {
	Server *DirectRelpServiceImpl
}

func (h DirectRelpHandler) HandleConnection(conn net.Conn, c conf.TCPSourceConfig) {
	// http://www.rsyslog.com/doc/relp.html
	config := conf.DirectRELPSourceConfig(c)
	s := h.Server
	s.AddConnection(conn)
	connID := s.forwarder.AddConn()
	scanner := bufio.NewScanner(conn)
	logger := s.Logger.New("ConnID", connID)

	defer func() {
		logger.Info("Scanning the DirectRELP stream has ended", "error", scanner.Err())
		s.forwarder.RemoveConn(connID)
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	var relpIsOpen bool

	client := ""
	path := ""
	remote := conn.RemoteAddr()

	var localPort int32
	if remote == nil {
		client = "localhost"
		localPort = 0
		path = conn.LocalAddr().String()
	} else {
		client = strings.Split(remote.String(), ":")[0]
		local := conn.LocalAddr()
		if local != nil {
			s := strings.Split(local.String(), ":")
			localPort, _ = utils.Atoi32(s[len(s)-1])
		}
	}
	client = strings.TrimSpace(client)
	path = strings.TrimSpace(path)
	localPortStr := strconv.FormatInt(int64(localPort), 10)

	logger = logger.New(
		"protocol", "directrelp",
		"client", client,
		"local_port", localPort,
		"unix_socket_path", path,
		"format", config.Format,
	)
	logger.Info("New client connection")
	base.ClientConnectionCounter.WithLabelValues("directrelp", client, localPortStr, path).Inc()

	s.wg.Add(1)
	go s.handleResponses(conn, connID, client, logger)

	timeout := config.Timeout
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner.Split(utils.RelpSplit)
	scanner.Buffer(make([]byte, 0, 132000), 132000)
	var rawmsg *model.RawTcpMessage
	var previous = int32(-1)

Loop:
	for scanner.Scan() {
		splits := bytes.SplitN(scanner.Bytes(), sp, 4)
		txnr, _ := utils.Atoi32(string(splits[0]))
		if txnr <= previous {
			logger.Warn("TXNR did not increase", "previous", previous, "current", txnr)
			relpProtocolErrorsCounter.WithLabelValues(client).Inc()
			return
		}
		previous = txnr
		command := string(splits[1])
		datalen, _ := strconv.Atoi(string(splits[2]))
		data := []byte{}
		if datalen != 0 {
			if len(splits) == 4 {
				data = bytes.Trim(splits[3], " \r\n")
			} else {
				logger.Warn("datalen is non-null, but no data is provided", "datalen", datalen)
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
		}
		switch command {
		case "open":
			if relpIsOpen {
				logger.Warn("Received open command twice")
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			fmt.Fprintf(conn, "%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, string(data))
			relpIsOpen = true
			logger.Info("Received 'open' command")
		case "close":
			if !relpIsOpen {
				logger.Warn("Received close command before open")
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			fmt.Fprintf(conn, "%d rsp 0\n0 serverclose 0\n", txnr)
			logger.Info("Received 'close' command")
			return
		case "syslog":
			if !relpIsOpen {
				logger.Warn("Received syslog command before open")
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			s.forwarder.Received(connID, txnr)
			if len(data) == 0 {
				s.forwarder.ForwardSucc(connID, txnr)
				continue Loop
			}
			if s.MaxMessageSize > 0 && len(data) > s.MaxMessageSize {
				logger.Warn("Message too large")
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			rawmsg = s.Pool.Get().(*model.RawTcpMessage)
			rawmsg.Txnr = txnr
			rawmsg.Client = client
			rawmsg.LocalPort = localPort
			rawmsg.UnixSocketPath = path
			rawmsg.ConfID = config.ConfID
			rawmsg.Encoding = config.Encoding
			rawmsg.Format = config.Format
			rawmsg.ConnID = connID
			rawmsg.Message = rawmsg.Message[:len(data)]
			copy(rawmsg.Message, data)
			err := s.rawMessagesQueue.Put(rawmsg)
			if err != nil {
				s.Logger.Error("Failed to enqueue new raw DirectRELP message", "error", err)
				return
			}
			base.IncomingMsgsCounter.WithLabelValues("directrelp", client, localPortStr, path).Inc()
			//logger.Debug("RELP client received a syslog message")
		default:
			logger.Warn("Unknown RELP command", "command", command)
			relpProtocolErrorsCounter.WithLabelValues(client).Inc()
			return
		}
		if timeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(timeout))
		}

	}
}
