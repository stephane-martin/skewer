package network

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sarama "github.com/Shopify/sarama"
	"github.com/oklog/ulid"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"golang.org/x/text/encoding"

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

var tr = true
var fa = false
var sp = []byte(" ")

type RelpServerStatus int

const (
	Stopped RelpServerStatus = iota
	Started
	FinalStopped
	Waiting
)

type ackForwarder struct {
	succ sync.Map
	fail sync.Map
	comm sync.Map
	next uintptr
}

func newAckForwarder() *ackForwarder {
	return &ackForwarder{}
}

func txnr2bytes(txnr int) []byte {
	bs := make([]byte, 8)
	ux := uint64(txnr) << 1
	if txnr < 0 {
		ux = ^ux
	}
	binary.LittleEndian.PutUint64(bs, ux)
	return bs
}

func bytes2txnr(b []byte) int {
	ux := binary.LittleEndian.Uint64(b)
	x := int64(ux >> 1)
	if ux&1 != 0 {
		x = ^x
	}
	return int(x)
}

func (f *ackForwarder) Received(connID uintptr, txnr int) {
	if c, ok := f.comm.Load(connID); ok {
		_ = c.(*queue.IntQueue).Put(txnr)
	}
}

func (f *ackForwarder) Commit(connID uintptr) {
	if c, ok := f.comm.Load(connID); ok {
		_, _ = c.(*queue.IntQueue).Get()
	}
}

func (f *ackForwarder) NextToCommit(connID uintptr) int {
	if c, ok := f.comm.Load(connID); ok {
		next, err := c.(*queue.IntQueue).Peek()
		if err != nil {
			return -1
		}
		return next
	}
	return -1
}

func (f *ackForwarder) ForwardSucc(connID uintptr, txnr int) {
	if q, ok := f.succ.Load(connID); ok {
		_ = q.(*queue.IntQueue).Put(txnr)
	}
}

func (f *ackForwarder) GetSucc(connID uintptr) int {
	if q, ok := f.succ.Load(connID); ok {
		txnr, err := q.(*queue.IntQueue).Get()
		if err != nil {
			return -1
		}
		return txnr
	}
	return -1
}

func (f *ackForwarder) ForwardFail(connID uintptr, txnr int) {
	if q, ok := f.fail.Load(connID); ok {
		_ = q.(*queue.IntQueue).Put(txnr)
	}
}

func (f *ackForwarder) GetFail(connID uintptr) int {
	if q, ok := f.fail.Load(connID); ok {
		txnr, err := q.(*queue.IntQueue).Get()
		if err != nil {
			return -1
		}
		return txnr
	}
	return -1
}

func (f *ackForwarder) AddConn() uintptr {
	connID := atomic.AddUintptr(&f.next, 1)
	f.succ.Store(connID, queue.NewIntQueue())
	f.fail.Store(connID, queue.NewIntQueue())
	f.comm.Store(connID, queue.NewIntQueue())
	return connID
}

func (f *ackForwarder) RemoveConn(connID uintptr) {
	if q, ok := f.succ.Load(connID); ok {
		q.(*queue.IntQueue).Dispose()
		f.succ.Delete(connID)
	}
	if q, ok := f.fail.Load(connID); ok {
		q.(*queue.IntQueue).Dispose()
		f.fail.Delete(connID)
	}
	f.comm.Delete(connID)
}

func (f *ackForwarder) RemoveAll() {
	f.succ = sync.Map{}
	f.fail = sync.Map{}
	f.comm = sync.Map{}
}

func (f *ackForwarder) Wait(connID uintptr) bool {
	qsucc, ok := f.succ.Load(connID)
	if !ok {
		return false
	}
	qfail, ok := f.fail.Load(connID)
	if !ok {
		return false
	}
	return queue.WaitOne(qsucc.(*queue.IntQueue), qfail.(*queue.IntQueue))
}

type meta struct {
	Txnr   int
	ConnID uintptr
}

type relpMetrics struct {
	IncomingMsgsCounter         *prometheus.CounterVec
	ClientConnectionCounter     *prometheus.CounterVec
	ParsingErrorCounter         *prometheus.CounterVec
	RelpAnswersCounter          *prometheus.CounterVec
	RelpProtocolErrorsCounter   *prometheus.CounterVec
	KafkaConnectionErrorCounter prometheus.Counter
	KafkaAckNackCounter         *prometheus.CounterVec
	MessageFilteringCounter     *prometheus.CounterVec
}

func NewRelpMetrics() *relpMetrics {
	m := &relpMetrics{}
	m.IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_incoming_messages_total",
			Help: "total number of messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)

	m.ClientConnectionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_client_connections_total",
			Help: "total number of client connections",
		},
		[]string{"protocol", "client", "port", "path"},
	)

	m.ParsingErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_parsing_errors_total",
			Help: "total number of times there was a parsing error",
		},
		[]string{"protocol", "client", "parser_name"},
	)

	m.RelpAnswersCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relp_answers_total",
			Help: "number of RELP rsp answers",
		},
		[]string{"status", "client"},
	)

	m.RelpProtocolErrorsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relp_protocol_errors_total",
			Help: "Number of RELP protocol errors",
		},
		[]string{"client"},
	)

	m.KafkaConnectionErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "skw_relp_kafka_connection_errors_total",
			Help: "number of kafka connection errors",
		},
	)

	m.KafkaAckNackCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relp_kafka_ack_total",
			Help: "number of kafka acknowledgments",
		},
		[]string{"status", "topic"},
	)

	m.MessageFilteringCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relp_messages_filtering_total",
			Help: "number of filtered messages by status",
		},
		[]string{"status", "client"},
	)
	return m
}

type RelpService struct {
	impl      *RelpServiceImpl
	QueueSize uint64
	logger    log15.Logger
	reporter  *base.Reporter
	direct    bool
	b         *binder.BinderClient
	sc        []conf.RelpSourceConfig
	pc        []conf.ParserConfig
	kc        conf.KafkaDestConfig
	wg        sync.WaitGroup
	gen       chan ulid.ULID
}

func NewRelpService(r *base.Reporter, gen chan ulid.ULID, b *binder.BinderClient, l log15.Logger) *RelpService {
	s := &RelpService{b: b, logger: l, reporter: r, direct: true, gen: gen}
	s.impl = NewRelpServiceImpl(s.direct, gen, r, s.b, s.logger)
	return s
}

func (s *RelpService) Gather() ([]*dto.MetricFamily, error) {
	return s.impl.registry.Gather()
}

func (s *RelpService) Start(test bool) (infos []model.ListenerInfo, err error) {
	// the Relp service manages registration in Consul by itself and
	// therefore does not report infos
	//if capabilities.CapabilitiesSupported {
	//	s.logger.Debug("Capabilities", "caps", capabilities.GetCaps())
	//}
	infos = []model.ListenerInfo{}
	s.impl = NewRelpServiceImpl(s.direct, s.gen, s.reporter, s.b, s.logger)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			state := <-s.impl.StatusChan
			switch state {
			case FinalStopped:
				//s.impl.Logger.Debug("The RELP service has been definitely halted")
				_ = s.reporter.Report([]model.ListenerInfo{})
				return

			case Stopped:
				//s.impl.Logger.Debug("The RELP service is stopped")
				s.impl.SetConf(s.sc, s.pc, s.kc, s.QueueSize)
				infos, err := s.impl.Start(test)
				if err == nil {
					err = s.reporter.Report(infos)
					if err != nil {
						// TODO
					}
				} else {
					err = s.reporter.Report([]model.ListenerInfo{})
					if err != nil {
						// TODO
					}
					s.impl.Logger.Warn("The RELP service has failed to start", "error", err)
					s.impl.StopAndWait()
				}

			case Waiting:
				//s.impl.Logger.Debug("RELP waiting")
				go func() {
					time.Sleep(time.Duration(30) * time.Second)
					s.impl.EndWait()
				}()

			case Started:
				//s.impl.Logger.Debug("The RELP service has been started")
			}
		}
	}()

	s.impl.StatusChan <- Stopped // trigger the RELP service to start
	return
}

func (s *RelpService) Shutdown() {
	s.Stop()
}

func (s *RelpService) Stop() {
	s.impl.FinalStop()
	s.wg.Wait()
}

func (s *RelpService) SetConf(sc []conf.RelpSourceConfig, pc []conf.ParserConfig, kc conf.KafkaDestConfig, direct bool, queueSize uint64) {
	s.sc = sc
	s.pc = pc
	s.kc = kc
	s.direct = direct
	s.QueueSize = queueSize
}

type RelpServiceImpl struct {
	StreamingService
	RelpConfigs         []conf.RelpSourceConfig
	kafkaConf           conf.KafkaDestConfig
	status              RelpServerStatus
	StatusChan          chan RelpServerStatus
	producer            sarama.AsyncProducer
	test                bool
	metrics             *relpMetrics
	registry            *prometheus.Registry
	reporter            *base.Reporter
	direct              bool
	gen                 chan ulid.ULID
	rawMessagesQueue    *tcp.Ring
	parsedMessagesQueue *queue.MessageQueue
	parsewg             sync.WaitGroup
	configs             map[ulid.ULID]conf.RelpSourceConfig
	forwarder           *ackForwarder
}

func NewRelpServiceImpl(direct bool, gen chan ulid.ULID, reporter *base.Reporter, b *binder.BinderClient, logger log15.Logger) *RelpServiceImpl {
	s := RelpServiceImpl{
		status:    Stopped,
		metrics:   NewRelpMetrics(),
		registry:  prometheus.NewRegistry(),
		reporter:  reporter,
		direct:    direct,
		gen:       gen,
		configs:   map[ulid.ULID]conf.RelpSourceConfig{},
		forwarder: newAckForwarder(),
	}
	s.StreamingService.init()
	s.registry.MustRegister(
		s.metrics.ClientConnectionCounter,
		s.metrics.IncomingMsgsCounter,
		s.metrics.KafkaAckNackCounter,
		s.metrics.KafkaConnectionErrorCounter,
		s.metrics.MessageFilteringCounter,
		s.metrics.ParsingErrorCounter,
		s.metrics.RelpAnswersCounter,
		s.metrics.RelpProtocolErrorsCounter,
	)
	s.StreamingService.BaseService.Logger = logger.New("class", "RelpServer")
	s.StreamingService.BaseService.Binder = b
	s.StreamingService.handler = RelpHandler{Server: &s}
	s.StatusChan = make(chan RelpServerStatus, 10)
	return &s
}

func (s *RelpServiceImpl) Start(test bool) ([]model.ListenerInfo, error) {
	s.LockStatus()
	defer s.UnlockStatus()
	if s.status == FinalStopped {
		return nil, errors.ServerDefinitelyStopped
	}
	if s.status != Stopped && s.status != Waiting {
		return nil, errors.ServerNotStopped
	}
	s.test = test

	infos := s.initTCPListeners()
	if len(infos) == 0 {
		s.Logger.Info("RELP service not started: no listener")
		return infos, nil
	}

	s.producer = nil
	if !s.test && s.direct {
		var err error
		s.producer, err = s.kafkaConf.GetAsyncProducer()
		if err != nil {
			s.resetTCPListeners()
			return nil, err
		}
	}

	s.Logger.Info("Listening on RELP", "nb_services", len(infos))

	s.parsedMessagesQueue = queue.NewMessageQueue()
	s.rawMessagesQueue = tcp.NewRing(s.QueueSize)
	s.configs = map[ulid.ULID]conf.RelpSourceConfig{}

	for _, l := range s.UnixListeners {
		s.configs[l.Conf.ConfID] = conf.RelpSourceConfig(l.Conf)
	}
	for _, l := range s.TcpListeners {
		s.configs[l.Conf.ConfID] = conf.RelpSourceConfig(l.Conf)
	}

	if !s.test && s.direct {
		s.wg.Add(1)
		go s.push2kafka()
		s.wg.Add(1)
		go s.handleKafkaResponses()
	}
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.parsewg.Add(1)
		go s.Parse()
	}

	s.status = Started
	s.StatusChan <- Started

	s.Listen()
	return infos, nil
}

func (s *RelpServiceImpl) Stop() {
	s.LockStatus()
	s.doStop(false, false)
	s.UnlockStatus()
}

func (s *RelpServiceImpl) FinalStop() {
	s.LockStatus()
	s.doStop(true, false)
	s.UnlockStatus()
}

func (s *RelpServiceImpl) StopAndWait() {
	s.LockStatus()
	s.doStop(false, true)
	s.UnlockStatus()
}

func (s *RelpServiceImpl) EndWait() {
	s.LockStatus()
	if s.status != Waiting {
		s.UnlockStatus()
		return
	}
	s.status = Stopped
	s.StatusChan <- Stopped
	s.UnlockStatus()
}

func (s *RelpServiceImpl) doStop(final bool, wait bool) {
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

func (s *RelpServiceImpl) SetConf(sc []conf.RelpSourceConfig, pc []conf.ParserConfig, kc conf.KafkaDestConfig, queueSize uint64) {
	tcpConfigs := []conf.TcpSourceConfig{}
	for _, c := range sc {
		tcpConfigs = append(tcpConfigs, conf.TcpSourceConfig(c))
	}
	s.StreamingService.SetConf(tcpConfigs, pc, queueSize, 132000)
	s.kafkaConf = kc
	s.BaseService.Pool = &sync.Pool{New: func() interface{} {
		return &model.RawTcpMessage{Message: make([]byte, 132000)}
	}}
}

func (s *RelpServiceImpl) Parse() {
	defer s.parsewg.Done()

	e := NewParsersEnv(s.ParserConfigs, s.Logger)

	var raw *model.RawTcpMessage
	var parser Parser
	var syslogMsg *model.SyslogMessage
	var parsedMsg model.FullMessage
	var err, f, nonf error
	var decoder *encoding.Decoder
	var logger log15.Logger

	for {
		raw, err = s.rawMessagesQueue.Get()
		if err != nil {
			return
		}
		if raw == nil {
			s.Logger.Error("rawMessagesQueue returns nil, should not happen!")
			return
		}

		logger = s.Logger.New(
			"protocol", "relp",
			"client", raw.Client,
			"local_port", raw.LocalPort,
			"unix_socket_path", raw.UnixSocketPath,
			"format", raw.Format,
			"txnr", raw.Txnr,
		)
		parser = e.GetParser(raw.Format)
		if parser == nil {
			s.forwarder.ForwardFail(raw.ConnID, raw.Txnr)
			logger.Crit("Unknown parser")
			s.Pool.Put(raw)
			return
		}
		decoder = utils.SelectDecoder(raw.Encoding)
		syslogMsg, err = parser.Parse(raw.Message[:raw.Size], decoder, raw.DontParseSD)
		if err != nil {
			logger.Warn("Parsing error", "message", string(raw.Message[:raw.Size]), "error", err)
			s.forwarder.ForwardFail(raw.ConnID, raw.Txnr)
			s.metrics.ParsingErrorCounter.WithLabelValues("relp", raw.Client, raw.Format).Inc()
			s.Pool.Put(raw)
			continue
		}
		if syslogMsg == nil {
			s.forwarder.ForwardSucc(raw.ConnID, raw.Txnr)
			s.Pool.Put(raw)
			continue
		}

		parsedMsg = model.FullMessage{
			Parsed: model.ParsedMessage{
				Fields:         *syslogMsg,
				Client:         raw.Client,
				LocalPort:      raw.LocalPort,
				UnixSocketPath: raw.UnixSocketPath,
			},
			Txnr:   raw.Txnr,
			ConfId: raw.ConfID,
			ConnID: raw.ConnID,
		}
		s.Pool.Put(raw)

		if s.direct {
			// send message directly to kafka
			_ = s.parsedMessagesQueue.Put(parsedMsg)
			continue
		}
		// else send message to the Store
		parsedMsg.Uid = <-s.gen
		f, nonf = s.reporter.Stash(parsedMsg)
		if f == nil && nonf == nil {
			s.forwarder.ForwardSucc(parsedMsg.ConnID, parsedMsg.Txnr)
		} else if f != nil {
			s.forwarder.ForwardFail(parsedMsg.ConnID, parsedMsg.Txnr)
			logger.Error("Fatal error pushing RELP message to the Store", "err", f)
			s.StopAndWait()
			return
		} else {
			s.forwarder.ForwardFail(parsedMsg.ConnID, parsedMsg.Txnr)
			logger.Warn("Non fatal error pushing RELP message to the Store", "err", nonf)
		}
	}

}

func (s *RelpServiceImpl) handleKafkaResponses() {
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
				s.metrics.KafkaAckNackCounter.WithLabelValues("ack", succ.Topic).Inc()
			} else {
				kafkaSuccChan = nil
			}
		case fail, more = <-kafkaFailChan:
			if more {
				metad := fail.Msg.Metadata.(meta)
				s.forwarder.ForwardFail(metad.ConnID, metad.Txnr)
				s.metrics.KafkaAckNackCounter.WithLabelValues("nack", fail.Msg.Topic).Inc()
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

func (s *RelpServiceImpl) handleResponses(conn net.Conn, connID uintptr, client string, logger log15.Logger) {
	defer func() {
		s.wg.Done()
	}()

	successes := map[int]bool{}
	failures := map[int]bool{}
	var err error

	writeSuccess := func(txnr int) (err error) {
		_, err = fmt.Fprintf(conn, "%d rsp 6 200 OK\n", txnr)
		return err
	}

	writeFailure := func(txnr int) (err error) {
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
					s.metrics.RelpAnswersCounter.WithLabelValues("200", client).Inc()
				}
			} else if failures[next] {
				err = writeFailure(next)
				if err == nil {
					//logger.Debug("NACK to client", "connid", connID, "txnr", next)
					delete(failures, next)
					s.metrics.RelpAnswersCounter.WithLabelValues("500", client).Inc()
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

func (s *RelpServiceImpl) push2kafka() {
	defer func() {
		s.producer.AsyncClose()
		s.wg.Done()
	}()
	envs := map[ulid.ULID]*javascript.Environment{}
	var e *javascript.Environment
	var haveEnv bool
	var message *model.FullMessage
	var topic string
	var partitionKey string
	var partitionNumber int32
	var errs []error
	var err error
	var logger log15.Logger
	var filterResult javascript.FilterResult
	var kafkaMsg *sarama.ProducerMessage
	var serialized []byte
	var reported time.Time
	var config conf.RelpSourceConfig

ForParsedChan:
	for s.parsedMessagesQueue.Wait(0) {
		message, err = s.parsedMessagesQueue.Get()
		if err != nil {
			// should not happen
			s.Logger.Error("Fatal error getting messages from the parsed messages queue", "error", err)
			s.StopAndWait()
			return
		}
		if message == nil {
			// should not happen
			continue ForParsedChan
		}
		logger = s.Logger.New("client", message.Parsed.Client, "port", message.Parsed.LocalPort, "path", message.Parsed.UnixSocketPath)
		e, haveEnv = envs[message.ConfId]
		if !haveEnv {
			config, haveEnv = s.configs[message.ConfId]
			if !haveEnv {
				s.Logger.Warn("Could not find the configuration for a message", "confId", message.ConfId, "txnr", message.Txnr)
				continue ForParsedChan
			}
			envs[message.ConfId] = javascript.NewFilterEnvironment(
				config.FilterFunc,
				config.TopicFunc,
				config.TopicTmpl,
				config.PartitionFunc,
				config.PartitionTmpl,
				config.PartitionNumberFunc,
				s.Logger,
			)
			e = envs[message.ConfId]
		}

		topic, errs = e.Topic(message.Parsed.Fields)
		for _, err = range errs {
			logger.Info("Error calculating topic", "error", err, "txnr", message.Txnr)
		}
		if len(topic) == 0 {
			logger.Warn("Topic or PartitionKey could not be calculated", "txnr", message.Txnr)
			s.forwarder.ForwardFail(message.ConnID, message.Txnr)
			continue ForParsedChan
		}
		partitionKey, errs = e.PartitionKey(message.Parsed.Fields)
		for _, err = range errs {
			logger.Info("Error calculating the partition key", "error", err, "txnr", message.Txnr)
		}
		partitionNumber, errs = e.PartitionNumber(message.Parsed.Fields)
		for _, err = range errs {
			logger.Info("Error calculating the partition number", "error", err, "txnr", message.Txnr)
		}

		filterResult, err = e.FilterMessage(&message.Parsed.Fields)

		switch filterResult {
		case javascript.DROPPED:
			s.forwarder.ForwardFail(message.ConnID, message.Txnr)
			s.metrics.MessageFilteringCounter.WithLabelValues("dropped", message.Parsed.Client).Inc()
			continue ForParsedChan
		case javascript.REJECTED:
			s.forwarder.ForwardFail(message.ConnID, message.Txnr)
			s.metrics.MessageFilteringCounter.WithLabelValues("rejected", message.Parsed.Client).Inc()
			continue ForParsedChan
		case javascript.PASS:
			s.metrics.MessageFilteringCounter.WithLabelValues("passing", message.Parsed.Client).Inc()
		default:
			s.forwarder.ForwardFail(message.ConnID, message.Txnr)
			s.metrics.MessageFilteringCounter.WithLabelValues("unknown", message.Parsed.Client).Inc()
			logger.Warn("Error happened processing message", "txnr", message.Txnr, "error", err)
			continue ForParsedChan
		}

		reported = time.Unix(0, message.Parsed.Fields.TimeReportedNum).UTC()
		message.Parsed.Fields.TimeGenerated = time.Unix(0, message.Parsed.Fields.TimeGeneratedNum).UTC().Format(time.RFC3339Nano)
		message.Parsed.Fields.TimeReported = reported.Format(time.RFC3339Nano)

		serialized, err = ffjson.Marshal(&message.Parsed)

		if err != nil {
			logger.Warn("Error generating Kafka message", "error", err, "txnr", message.Txnr)
			s.forwarder.ForwardFail(message.ConnID, message.Txnr)
			continue ForParsedChan
		}

		kafkaMsg = &sarama.ProducerMessage{
			Key:       sarama.StringEncoder(partitionKey),
			Partition: partitionNumber,
			Value:     sarama.ByteEncoder(serialized),
			Topic:     topic,
			Timestamp: reported,
			Metadata:  meta{Txnr: message.Txnr, ConnID: message.ConnID},
		}

		if s.test {
			// "fake" send messages to kafka
			fmt.Fprintf(os.Stderr, "pkey: '%s' topic:'%s' txnr:'%d'\n", partitionKey, topic, message.Txnr)
			fmt.Fprintln(os.Stderr, string(serialized))
			fmt.Fprintln(os.Stderr)
			s.forwarder.ForwardSucc(message.ConnID, message.Txnr)
		} else {
			// send messages to Kafka
			s.producer.Input() <- kafkaMsg
		}
	}

}

type RelpHandler struct {
	Server *RelpServiceImpl
}

func (h RelpHandler) HandleConnection(conn net.Conn, c conf.TcpSourceConfig) {
	// http://www.rsyslog.com/doc/relp.html
	config := conf.RelpSourceConfig(c)
	s := h.Server
	s.AddConnection(conn)
	connID := s.forwarder.AddConn()
	scanner := bufio.NewScanner(conn)
	logger := s.Logger.New("ConnID", connID)

	defer func() {
		logger.Info("Scanning the RELP stream has ended", "error", scanner.Err())
		s.forwarder.RemoveConn(connID)
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	var relpIsOpen bool

	client := ""
	path := ""
	remote := conn.RemoteAddr()

	var localPort int
	if remote == nil {
		client = "localhost"
		localPort = 0
		path = conn.LocalAddr().String()
	} else {
		client = strings.Split(remote.String(), ":")[0]
		local := conn.LocalAddr()
		if local != nil {
			s := strings.Split(local.String(), ":")
			localPort, _ = strconv.Atoi(s[len(s)-1])
		}
	}
	client = strings.TrimSpace(client)
	path = strings.TrimSpace(path)
	localPortStr := strconv.FormatInt(int64(localPort), 10)

	logger = logger.New(
		"protocol", "relp",
		"client", client,
		"local_port", localPort,
		"unix_socket_path", path,
		"format", config.Format,
	)
	logger.Info("New client connection")
	s.metrics.ClientConnectionCounter.WithLabelValues("relp", client, localPortStr, path).Inc()

	s.wg.Add(1)
	go s.handleResponses(conn, connID, client, logger)

	timeout := config.Timeout
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner.Split(utils.RelpSplit)
	scanner.Buffer(make([]byte, 0, 132000), 132000)
	var rawmsg *model.RawTcpMessage
	var previous = int(-1)

Loop:
	for scanner.Scan() {
		splits := bytes.SplitN(scanner.Bytes(), sp, 4)
		txnr, _ := strconv.Atoi(string(splits[0]))
		if txnr <= previous {
			logger.Warn("TXNR did not increase", "previous", previous, "current", txnr)
			s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
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
				s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
		}
		switch command {
		case "open":
			if relpIsOpen {
				logger.Warn("Received open command twice")
				s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			fmt.Fprintf(conn, "%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, string(data))
			relpIsOpen = true
			logger.Info("Received 'open' command")
		case "close":
			if !relpIsOpen {
				logger.Warn("Received close command before open")
				s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			fmt.Fprintf(conn, "%d rsp 0\n0 serverclose 0\n", txnr)
			logger.Info("Received 'close' command")
			return
		case "syslog":
			if !relpIsOpen {
				logger.Warn("Received syslog command before open")
				s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			s.forwarder.Received(connID, txnr)
			if len(data) == 0 {
				s.forwarder.ForwardSucc(connID, txnr)
				continue Loop
			}
			rawmsg = s.Pool.Get().(*model.RawTcpMessage)
			rawmsg.Size = len(data)
			rawmsg.Txnr = txnr
			rawmsg.Client = client
			rawmsg.LocalPort = localPort
			rawmsg.UnixSocketPath = path
			rawmsg.ConfID = config.ConfID
			rawmsg.DontParseSD = config.DontParseSD
			rawmsg.Encoding = config.Encoding
			rawmsg.Format = config.Format
			rawmsg.ConnID = connID
			copy(rawmsg.Message, data)
			err := s.rawMessagesQueue.Put(rawmsg)
			if err != nil {
				s.Logger.Error("Failed to enqueue new raw RELP message", "error", err)
				return
			}
			s.metrics.IncomingMsgsCounter.WithLabelValues("relp", client, localPortStr, path).Inc()
			//logger.Debug("RELP client received a syslog message")
		default:
			logger.Warn("Unknown RELP command", "command", command)
			s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
			return
		}
		if timeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(timeout))
		}

	}
}
