package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
)

type RelpServerStatus int

const (
	Stopped RelpServerStatus = iota
	Started
	FinalStopped
	Waiting
)

type RelpServer struct {
	StreamServer
	status      RelpServerStatus
	StatusChan  chan RelpServerStatus
	kafkaClient sarama.Client
	metrics     *metrics.Metrics
	test        bool
}

func (s *RelpServer) init() {
	s.StreamServer.init()
}

func NewRelpServer(c *conf.GConfig, test bool, metrics *metrics.Metrics, logger log15.Logger) *RelpServer {
	s := RelpServer{status: Stopped, metrics: metrics, test: test}
	s.logger = logger.New("class", "RelpServer")
	s.init()
	s.protocol = "relp"
	s.Conf = *c
	s.handler = RelpHandler{Server: &s}
	s.StatusChan = make(chan RelpServerStatus, 10)
	return &s
}

func (s *RelpServer) Start() error {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status == FinalStopped {
		return ServerDefinitelyStopped
	}
	if s.status != Stopped && s.status != Waiting {
		return ServerNotStopped
	}

	nb := s.initTCPListeners()
	if nb == 0 {
		s.logger.Debug("RELP service not started: no listening port")
		return nil
	} else {
		s.logger.Info("Listening on RELP", "nb_services", nb)
	}
	if !s.test {
		var err error
		s.kafkaClient, err = s.Conf.Kafka.GetClient()
		if err != nil {
			// sarama/kafka error
			s.resetTCPListeners()
			return err
		}
	}

	s.status = Started
	s.StatusChan <- Started

	s.Listen()
	return nil
}

func (s *RelpServer) Stop() {
	s.doStop(false, false, s.statusMutex)
}

func (s *RelpServer) FinalStop() {
	s.doStop(true, false, s.statusMutex)
}

func (s *RelpServer) StopAndWait() {
	s.doStop(false, true, s.statusMutex)
}

func (s *RelpServer) EndWait() {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != Waiting {
		return
	}
	s.status = Stopped
	s.StatusChan <- Stopped
}

func (s *RelpServer) doStop(final bool, wait bool, mu *sync.Mutex) {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
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
	Server *RelpServer
}

func (h RelpHandler) HandleConnection(conn net.Conn, config conf.SyslogConfig) {
	// http://www.rsyslog.com/doc/relp.html

	var local_port int
	var err error

	s := h.Server
	s.AddConnection(conn)

	raw_messages_chan := make(chan *model.RelpRawMessage)
	parsed_messages_chan := make(chan *model.RelpParsedMessage)
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

	logger := s.logger.New("protocol", s.protocol, "client", client, "local_port", local_port, "unix_socket_path", path, "format", config.Format)
	logger.Info("New client connection")
	s.metrics.ClientConnectionCounter.WithLabelValues(s.protocol, client, local_port_s, path).Inc()

	// pull messages from raw_messages_chan and push them to parsed_messages_chan
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		e := NewParsersEnv(s.Conf.Parsers, s.logger)
		for m := range raw_messages_chan {

			parser := e.GetParser(config.Format)
			if parser == nil {
				logger.Error("Unknown parser")
				continue
			}
			p, err := parser.Parse(m.Raw.Message, config.DontParseSD)
			if err == nil {
				parsed_msg := model.RelpParsedMessage{
					Parsed: &model.ParsedMessage{
						Fields:         p,
						Client:         m.Raw.Client,
						LocalPort:      m.Raw.LocalPort,
						UnixSocketPath: m.Raw.UnixSocketPath,
					},
					Txnr: m.Txnr,
				}
				parsed_messages_chan <- &parsed_msg
			} else {
				s.metrics.ParsingErrorCounter.WithLabelValues(config.Format, client).Inc()
				logger.Warn("Parsing error", "message", m.Raw.Message, "error", err)
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
						s.metrics.RelpAnswersCounter.WithLabelValues("200", client).Inc()
					} else {
						other_successes_chan = nil
					}
				case other_txnr, more := <-other_fails_chan:
					if more {
						answer := fmt.Sprintf("%d rsp 6 500 KO\n", other_txnr)
						conn.Write([]byte(answer))
						s.metrics.RelpAnswersCounter.WithLabelValues("500", client).Inc()
					} else {
						other_fails_chan = nil
					}
				}
			}

		}()
	} else {
		producer, err = s.Conf.Kafka.GetAsyncProducer()
		if err != nil {
			s.metrics.KafkaConnectionErrorCounter.Inc()
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
			successes := map[int]bool{}
			failures := map[int]bool{}
			successChan := producer.Successes()
			failureChan := producer.Errors()
			last_committed_txnr := 0

			for {
				if successChan == nil && failureChan == nil && other_successes_chan == nil && other_fails_chan == nil {
					return
				}
				select {
				case succ, more := <-successChan:
					if more {
						// forward the ACK to rsyslog
						txnr := succ.Metadata.(int)
						successes[txnr] = true
						s.metrics.KafkaAckNackCounter.WithLabelValues("ack", succ.Topic).Inc()
					} else {
						successChan = nil
					}
				case fail, more := <-failureChan:
					if more {
						txnr := fail.Msg.Metadata.(int)
						failures[txnr] = true
						logger.Info("NACK from Kafka", "error", fail.Error(), "txnr", txnr, "topic", fail.Msg.Topic)
						fatal = model.IsFatalKafkaError(fail.Err)
						s.metrics.KafkaAckNackCounter.WithLabelValues("nack", fail.Msg.Topic).Inc()
					} else {
						failureChan = nil
					}
				case other_txnr, more := <-other_successes_chan:
					if more {
						successes[other_txnr] = true
					} else {
						other_successes_chan = nil
					}
				case other_txnr, more := <-other_fails_chan:
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
						s.metrics.RelpAnswersCounter.WithLabelValues("200", client).Inc()
					} else if _, ok := failures[last_committed_txnr+1]; ok {
						last_committed_txnr++
						delete(failures, last_committed_txnr)
						answer := fmt.Sprintf("%d rsp 6 500 KO\n", last_committed_txnr)
						conn.Write([]byte(answer))
						s.metrics.RelpAnswersCounter.WithLabelValues("500", client).Inc()
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
		e := javascript.NewFilterEnvironment(config.FilterFunc, config.TopicFunc, config.TopicTmpl, config.PartitionFunc, config.PartitionTmpl, s.logger)

	ForParsedChan:
		for m := range parsed_messages_chan {
			topic, errs := e.Topic(m.Parsed.Fields)
			for _, err := range errs {
				logger.Info("Error calculating topic", "error", err, "txnr", m.Txnr)
			}
			partitionKey, errs := e.PartitionKey(m.Parsed.Fields)
			for _, err := range errs {
				logger.Info("Error calculating the partition key", "error", err, "txnr", m.Txnr)
			}

			if len(topic) == 0 || len(partitionKey) == 0 {
				logger.Warn("Topic or PartitionKey could not be calculated", "txnr", m.Txnr)
				other_fails_chan <- m.Txnr
				continue ForParsedChan
			}

			tmsg, filterResult, err := e.FilterMessage(m.Parsed.Fields)

			switch filterResult {
			case javascript.DROPPED:
				other_successes_chan <- m.Txnr
				s.metrics.MessageFilteringCounter.WithLabelValues("dropped", client).Inc()
				continue ForParsedChan
			case javascript.REJECTED:
				other_fails_chan <- m.Txnr
				s.metrics.MessageFilteringCounter.WithLabelValues("rejected", client).Inc()
				continue ForParsedChan
			case javascript.PASS:
				s.metrics.MessageFilteringCounter.WithLabelValues("passing", client).Inc()
				if tmsg == nil {
					other_successes_chan <- m.Txnr
					continue ForParsedChan
				}
			default:
				other_fails_chan <- m.Txnr
				content, _ := json.Marshal(m.Parsed.Fields)
				logger.Warn("Error happened processing message", "txnr", m.Txnr, "message", content, "error", err)
				s.metrics.MessageFilteringCounter.WithLabelValues("unknown", client).Inc()
				continue ForParsedChan
			}

			nmsg := model.ParsedMessage{
				Fields:    tmsg,
				Client:    m.Parsed.Client,
				LocalPort: m.Parsed.LocalPort,
			}

			kafkaMsg, err := nmsg.ToKafkaMessage(partitionKey, topic)
			if err != nil {
				logger.Warn("Error generating Kafka message", "error", err, "txnr", m.Txnr)
				other_fails_chan <- m.Txnr
				continue ForParsedChan
			}
			kafkaMsg.Metadata = m.Txnr

			if s.test {
				v, _ := kafkaMsg.Value.Encode()
				pkey, _ := kafkaMsg.Key.Encode()
				fmt.Printf("pkey: '%s' topic:'%s' txnr:'%d'\n", pkey, kafkaMsg.Topic, m.Txnr)
				fmt.Println(string(v))
				fmt.Println()
				other_successes_chan <- m.Txnr
			} else {
				producer.Input() <- kafkaMsg
			}
		}
	}()

	timeout := config.Timeout
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner := bufio.NewScanner(conn)
	scanner.Split(RelpSplit)
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
					s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
					return
				}
				answer := fmt.Sprintf("%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, data)
				conn.Write([]byte(answer))
				relpIsOpen = true
				logger.Info("Received 'open' command")
			case "close":
				if !relpIsOpen {
					logger.Warn("Received close command before open")
					s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
					return
				}
				answer := fmt.Sprintf("%d rsp 0\n0 serverclose 0\n", txnr)
				conn.Write([]byte(answer))
				relpIsOpen = false
				logger.Info("Received 'close' command")
			case "syslog":
				if !relpIsOpen {
					logger.Warn("Received syslog command before open")
					s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
					return
				}
				raw := model.RelpRawMessage{
					Txnr: txnr,
					Raw: &model.RawMessage{
						Message:   data,
						Client:    client,
						LocalPort: local_port,
					},
				}
				s.metrics.IncomingMsgsCounter.WithLabelValues(s.protocol, client, local_port_s, path).Inc()
				raw_messages_chan <- &raw
			default:
				logger.Warn("Unknown RELP command", "command", command)
				s.metrics.RelpProtocolErrorsCounter.WithLabelValues(client).Inc()
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
