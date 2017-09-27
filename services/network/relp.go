package network

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pquerna/ffjson/ffjson"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/services/errors"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/utils"
)

type RelpServerStatus int

const (
	Stopped RelpServerStatus = iota
	Started
	FinalStopped
	Waiting
)

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
			Help: "total number of syslog messages that were received",
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
	impl     *RelpServiceImpl
	logger   log15.Logger
	reporter *base.Reporter
	b        *binder.BinderClient
	sc       []conf.SyslogConfig
	pc       []conf.ParserConfig
	kc       conf.KafkaConfig
	wg       *sync.WaitGroup
}

func NewRelpService(r *base.Reporter, b *binder.BinderClient, l log15.Logger) *RelpService {
	s := &RelpService{b: b, logger: l, reporter: r, wg: &sync.WaitGroup{}}
	s.impl = NewRelpServiceImpl(s.b, s.logger)
	return s
}

func (s *RelpService) Gather() ([]*dto.MetricFamily, error) {
	return s.impl.registry.Gather()
}

func (s *RelpService) Start(test bool) (infos []model.ListenerInfo, err error) {
	// the Relp service manages registration in Consul by itself and
	// therefore does not report infos
	if capabilities.CapabilitiesSupported {
		s.logger.Debug("Capabilities", "caps", capabilities.GetCaps())
	}
	infos = []model.ListenerInfo{}
	s.impl = NewRelpServiceImpl(s.b, s.logger)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			state := <-s.impl.StatusChan
			switch state {
			case FinalStopped:
				s.impl.Logger.Debug("The RELP service has been definitely halted")
				s.reporter.Report([]model.ListenerInfo{})
				return

			case Stopped:
				s.impl.Logger.Debug("The RELP service is stopped")
				s.impl.SetConf(s.sc, s.pc)
				s.impl.SetKafkaConf(s.kc)
				infos, err := s.impl.Start(test)
				if err == nil {
					s.reporter.Report(infos)
				} else {
					s.reporter.Report([]model.ListenerInfo{})
					s.impl.Logger.Warn("The RELP service has failed to start", "error", err)
					s.impl.StopAndWait()
				}

			case Waiting:
				s.impl.Logger.Debug("RELP waiting")
				go func() {
					time.Sleep(time.Duration(30) * time.Second)
					s.impl.EndWait()
				}()

			case Started:
				s.impl.Logger.Debug("The RELP service has been started")
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

func (s *RelpService) SetConf(sc []conf.SyslogConfig, pc []conf.ParserConfig, kc conf.KafkaConfig) {
	s.sc = sc
	s.pc = pc
	s.kc = kc
}

type RelpServiceImpl struct {
	StreamingService
	kafkaConf   conf.KafkaConfig
	status      RelpServerStatus
	StatusChan  chan RelpServerStatus
	kafkaClient sarama.Client
	test        bool
	metrics     *relpMetrics
	registry    *prometheus.Registry
}

func NewRelpServiceImpl(b *binder.BinderClient, logger log15.Logger) *RelpServiceImpl {
	s := RelpServiceImpl{status: Stopped, metrics: NewRelpMetrics(), registry: prometheus.NewRegistry()}
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
	s.StreamingService.BaseService.Protocol = "relp"
	s.StreamingService.handler = RelpHandler{Server: &s}
	s.StatusChan = make(chan RelpServerStatus, 10)
	return &s
}

func (s *RelpServiceImpl) SetKafkaConf(c conf.KafkaConfig) {
	s.kafkaConf = c
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
		s.Logger.Debug("RELP service not started: no listener")
		return infos, nil
	} else {
		s.Logger.Info("Listening on RELP", "nb_services", len(infos))
	}

	if !s.test {
		var err error
		s.kafkaClient, err = s.kafkaConf.GetClient()
		if err != nil {
			// sarama/kafka error
			s.resetTCPListeners()
			return nil, err
		}
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

	s.resetTCPListeners()
	// wait that all goroutines have ended
	s.wg.Wait()

	if s.kafkaClient != nil {
		s.kafkaClient.Close()
		s.kafkaClient = nil
	}

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

type RelpHandler struct {
	Server *RelpServiceImpl
}

func (h RelpHandler) HandleConnection(conn net.Conn, config *conf.SyslogConfig) {
	// http://www.rsyslog.com/doc/relp.html

	var local_port int
	var err error

	s := h.Server
	s.AddConnection(conn)

	raw_messages_chan := make(chan model.RelpRawMessage)
	parsed_messages_chan := make(chan model.RelpParsedMessage)
	other_successes_chan := make(chan int)
	other_fails_chan := make(chan int)

	relpIsOpen := false

	client := ""
	path := ""
	remote := conn.RemoteAddr()

	if remote == nil {
		client = "localhost"
		local_port = 0
		path = conn.LocalAddr().String()
	} else {
		client = strings.Split(remote.String(), ":")[0]
		local := conn.LocalAddr()
		if local != nil {
			s := strings.Split(local.String(), ":")
			local_port, _ = strconv.Atoi(s[len(s)-1])
		}
	}
	client = strings.TrimSpace(client)
	path = strings.TrimSpace(path)
	local_port_s := strconv.FormatInt(int64(local_port), 10)

	logger := s.Logger.New("protocol", s.Protocol, "client", client, "local_port", local_port, "unix_socket_path", path, "format", config.Format)
	logger.Info("New client connection")
	if s.metrics != nil {
		s.metrics.ClientConnectionCounter.WithLabelValues(s.Protocol, client, local_port_s, path).Inc()
	}

	// pull messages from raw_messages_chan and push them to parsed_messages_chan
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		e := NewParsersEnv(s.ParserConfigs, s.Logger)
		parser := e.GetParser(config.Format)
		if parser == nil {
			logger.Crit("Unknown parser")
			return
		}

		var raw model.RelpRawMessage
		var syslogMsg model.SyslogMessage
		var err error
		var parsedMsg model.RelpParsedMessage
		decoder := utils.SelectDecoder(config.Encoding)

		for raw = range raw_messages_chan {
			syslogMsg, err = parser.Parse(raw.Raw.Message, decoder, config.DontParseSD)
			if err == nil {
				parsedMsg = model.RelpParsedMessage{
					Parsed: model.ParsedMessage{
						Fields:         syslogMsg,
						Client:         raw.Raw.Client,
						LocalPort:      raw.Raw.LocalPort,
						UnixSocketPath: raw.Raw.UnixSocketPath,
					},
					Txnr: raw.Txnr,
				}
				parsed_messages_chan <- parsedMsg
			} else if s.metrics != nil {
				s.metrics.ParsingErrorCounter.WithLabelValues(s.Protocol, client, config.Format).Inc()
				logger.Warn("Parsing error", "message", raw.Raw.Message, "error", err)
			}
		}
		close(parsed_messages_chan)
	}()

	defer func() {
		// closing raw_messages_chan causes parsed_messages_chan to be closed too, because of the goroutine just above
		close(raw_messages_chan)
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	var producer sarama.AsyncProducer

	if s.test {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			for {
				if other_successes_chan == nil && other_fails_chan == nil {
					return
				}
				select {
				case other_txnr, more := <-other_successes_chan:
					if more {
						answer := fmt.Sprintf("%d rsp 6 200 OK\n", other_txnr)
						conn.Write([]byte(answer))
						if s.metrics != nil {
							s.metrics.RelpAnswersCounter.WithLabelValues("200", client).Inc()
						}
					} else {
						other_successes_chan = nil
					}
				case other_txnr, more := <-other_fails_chan:
					if more {
						answer := fmt.Sprintf("%d rsp 6 500 KO\n", other_txnr)
						conn.Write([]byte(answer))
						if s.metrics != nil {
							s.metrics.RelpAnswersCounter.WithLabelValues("500", client).Inc()
						}
					} else {
						other_fails_chan = nil
					}
				}
			}

		}()
	} else {
		producer, err = s.kafkaConf.GetAsyncProducer()
		if err != nil {
			if s.metrics != nil {
				s.metrics.KafkaConnectionErrorCounter.Inc()
			}
			logger.Warn("Can't get a kafka producer. Aborting handleConn.")
			return
		}
		// AsyncClose will eventually terminate the goroutine just below
		defer producer.AsyncClose()

		// listen for the ACKs coming from Kafka
		// this goroutine ends after producer.AsyncClose() is called
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			fatal := false
			more := false
			successes := map[int]bool{}
			failures := map[int]bool{}
			successChan := producer.Successes()
			failureChan := producer.Errors()
			var last_committed_txnr int
			var other_txnr int
			var succ *sarama.ProducerMessage
			var fail *sarama.ProducerError

			for {
				if successChan == nil && failureChan == nil && other_successes_chan == nil && other_fails_chan == nil {
					return
				}
				select {
				case succ, more = <-successChan:
					if more {
						// forward the ACK to rsyslog
						txnr := succ.Metadata.(int)
						successes[txnr] = true
						if s.metrics != nil {
							s.metrics.KafkaAckNackCounter.WithLabelValues("ack", succ.Topic).Inc()
						}
					} else {
						successChan = nil
					}
				case fail, more = <-failureChan:
					if more {
						txnr := fail.Msg.Metadata.(int)
						failures[txnr] = true
						logger.Info("NACK from Kafka", "error", fail.Error(), "txnr", txnr, "topic", fail.Msg.Topic)
						fatal = model.IsFatalKafkaError(fail.Err)
						if s.metrics != nil {
							s.metrics.KafkaAckNackCounter.WithLabelValues("nack", fail.Msg.Topic).Inc()
						}
					} else {
						failureChan = nil
					}
				case other_txnr, more = <-other_successes_chan:
					if more {
						successes[other_txnr] = true
					} else {
						other_successes_chan = nil
					}
				case other_txnr, more = <-other_fails_chan:
					if more {
						failures[other_txnr] = true
					} else {
						other_fails_chan = nil
					}
				}

				// rsyslog expects the ACK/txnr correctly and monotonously ordered
				// so we need a bit of cooking to ensure that
				for {
					if _, ok := successes[last_committed_txnr+1]; ok {
						last_committed_txnr++
						delete(successes, last_committed_txnr)
						answer := fmt.Sprintf("%d rsp 6 200 OK\n", last_committed_txnr)
						conn.Write([]byte(answer))
						if s.metrics != nil {
							s.metrics.RelpAnswersCounter.WithLabelValues("200", client).Inc()
						}
					} else if _, ok := failures[last_committed_txnr+1]; ok {
						last_committed_txnr++
						delete(failures, last_committed_txnr)
						answer := fmt.Sprintf("%d rsp 6 500 KO\n", last_committed_txnr)
						conn.Write([]byte(answer))
						if s.metrics != nil {
							s.metrics.RelpAnswersCounter.WithLabelValues("500", client).Inc()
						}
					} else {
						break
					}
				}

				if fatal {
					s.StopAndWait()
					return
				}
			}
		}()
	}

	// push parsed messages to Kafka
	s.wg.Add(1)
	go func() {
		defer func() {
			close(other_successes_chan)
			close(other_fails_chan)
			s.wg.Done()
		}()

		e := javascript.NewFilterEnvironment(config.FilterFunc, config.TopicFunc, config.TopicTmpl, config.PartitionFunc, config.PartitionTmpl, s.Logger)
		var message model.RelpParsedMessage
		var topic string
		var partitionKey string
		var errs []error
		var err error
		var filterResult javascript.FilterResult
		var kafkaMsg *sarama.ProducerMessage
		var serialized []byte
		var reported time.Time

	ForParsedChan:
		for message = range parsed_messages_chan {
			topic, errs = e.Topic(message.Parsed.Fields)
			for _, err = range errs {
				logger.Info("Error calculating topic", "error", err, "txnr", message.Txnr)
			}
			partitionKey, errs = e.PartitionKey(message.Parsed.Fields)
			for _, err = range errs {
				logger.Info("Error calculating the partition key", "error", err, "txnr", message.Txnr)
			}

			if len(topic) == 0 || len(partitionKey) == 0 {
				logger.Warn("Topic or PartitionKey could not be calculated", "txnr", message.Txnr)
				other_fails_chan <- message.Txnr
				continue ForParsedChan
			}
			filterResult, err = e.FilterMessage(&message.Parsed.Fields)

			switch filterResult {
			case javascript.DROPPED:
				other_successes_chan <- message.Txnr
				s.metrics.MessageFilteringCounter.WithLabelValues("dropped", client).Inc()
				continue ForParsedChan
			case javascript.REJECTED:
				other_fails_chan <- message.Txnr
				s.metrics.MessageFilteringCounter.WithLabelValues("rejected", client).Inc()
				continue ForParsedChan
			case javascript.PASS:
				s.metrics.MessageFilteringCounter.WithLabelValues("passing", client).Inc()
			default:
				other_fails_chan <- message.Txnr
				logger.Warn("Error happened processing message", "txnr", message.Txnr, "error", err)
				s.metrics.MessageFilteringCounter.WithLabelValues("unknown", client).Inc()
				continue ForParsedChan
			}

			reported = time.Unix(0, message.Parsed.Fields.TimeReportedNum).UTC()
			message.Parsed.Fields.TimeGenerated = time.Unix(0, message.Parsed.Fields.TimeGeneratedNum).UTC().Format(time.RFC3339Nano)
			message.Parsed.Fields.TimeReported = reported.Format(time.RFC3339Nano)

			serialized, err = ffjson.Marshal(&message.Parsed)

			if err != nil {
				logger.Warn("Error generating Kafka message", "error", err, "txnr", message.Txnr)
				other_fails_chan <- message.Txnr
				continue ForParsedChan
			}

			kafkaMsg = &sarama.ProducerMessage{
				Key:       sarama.StringEncoder(partitionKey),
				Value:     sarama.ByteEncoder(serialized),
				Topic:     topic,
				Timestamp: reported,
				Metadata:  message.Txnr,
			}

			if s.test {
				v, _ := kafkaMsg.Value.Encode()
				pkey, _ := kafkaMsg.Key.Encode()
				fmt.Fprintf(os.Stderr, "pkey: '%s' topic:'%s' txnr:'%d'\n", pkey, kafkaMsg.Topic, message.Txnr)
				fmt.Fprintln(os.Stderr, string(v))
				fmt.Fprintln(os.Stderr)
				other_successes_chan <- message.Txnr
			} else {
				producer.Input() <- kafkaMsg
			}
			ffjson.Pool(serialized)
		}
	}()

	timeout := config.Timeout
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner := bufio.NewScanner(conn)
	scanner.Split(RelpSplit)
	var raw model.RelpRawMessage
	for {
		if scanner.Scan() {
			if timeout > 0 {
				conn.SetReadDeadline(time.Now().Add(timeout))
			}
			line := scanner.Text()
			splits := strings.SplitN(line, " ", 4)
			txnr, _ := strconv.Atoi(splits[0])
			command := splits[1]
			datalen, _ := strconv.Atoi(splits[2])
			data := ""
			if datalen != 0 {
				data = strings.Trim(splits[3], " \r\n")
			}
			switch command {
			case "open":
				if relpIsOpen {
					logger.Warn("Received open command twice")
					if s.metrics != nil {
						s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
					}
					return
				}
				answer := fmt.Sprintf("%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, data)
				conn.Write([]byte(answer))
				relpIsOpen = true
				logger.Info("Received 'open' command")
			case "close":
				if !relpIsOpen {
					logger.Warn("Received close command before open")
					if s.metrics != nil {
						s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
					}
					return
				}
				answer := fmt.Sprintf("%d rsp 0\n0 serverclose 0\n", txnr)
				conn.Write([]byte(answer))
				relpIsOpen = false
				logger.Info("Received 'close' command")
			case "syslog":
				if !relpIsOpen {
					logger.Warn("Received syslog command before open")
					if s.metrics != nil {
						s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
					}
					return
				}
				raw = model.RelpRawMessage{
					Txnr: txnr,
					Raw: model.RawMessage{
						Message:   []byte(data),
						Client:    client,
						LocalPort: local_port,
					},
				}
				if s.metrics != nil {
					s.metrics.IncomingMsgsCounter.WithLabelValues(s.Protocol, client, local_port_s, path).Inc()
				}
				raw_messages_chan <- raw
			default:
				logger.Warn("Unknown RELP command", "command", command)
				if s.metrics != nil {
					s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
				}
				return
			}
		} else {
			logger.Info("Scanning the RELP stream has ended", "error", scanner.Err())
			return
		}
	}
}

func splitSpaceOrLF(r rune) bool {
	return r == ' ' || r == '\n' || r == '\r'
}

// RelpSplit is used to extract RELP lines from the incoming TCP stream
func RelpSplit(data []byte, atEOF bool) (int, []byte, error) {
	trimmed_data := bytes.TrimLeft(data, " \r\n")
	if len(trimmed_data) == 0 {
		return 0, nil, nil
	}
	splits := bytes.FieldsFunc(trimmed_data, splitSpaceOrLF)
	l := len(splits)
	if l < 3 {
		// Request more data
		return 0, nil, nil
	}

	txnr_s := string(splits[0])
	command := string(splits[1])
	datalen_s := string(splits[2])
	token_s := txnr_s + " " + command + " " + datalen_s
	advance := len(data) - len(trimmed_data) + len(token_s) + 1

	if l == 3 && (len(data) < advance) {
		// datalen field is not complete, request more data
		return 0, nil, nil
	}

	_, err := strconv.Atoi(txnr_s)
	if err != nil {
		return 0, nil, err
	}
	datalen, err := strconv.Atoi(datalen_s)
	if err != nil {
		return 0, nil, err
	}
	if datalen == 0 {
		return advance, []byte(token_s), nil
	}
	advance += datalen + 1
	if len(data) >= advance {
		token := bytes.Trim(data[:advance], " \r\n")
		return advance, token, nil
	}
	// Request more data
	return 0, nil, nil
}
