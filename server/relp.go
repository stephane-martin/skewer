package server

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/model"
)

type RelpServerStatus int

const (
	Stopped RelpServerStatus = iota
	Started
	FinalStopped
	Waiting
)

type RelpServer struct {
	Server
	statusMutex sync.Mutex
	status      RelpServerStatus
	StatusChan  chan RelpServerStatus
}

func NewRelpServer(c *conf.GlobalConfig, logger log15.Logger) *RelpServer {
	s := RelpServer{}
	s.protocol = "relp"
	s.stream = true
	s.Conf = c
	s.listeners = map[int]net.Listener{}
	s.connections = map[net.Conn]bool{}
	s.logger = logger.New("class", "RelpServer")
	s.handler = RelpHandler{Server: &s}

	s.StatusChan = make(chan RelpServerStatus, 1)
	s.status = Stopped
	return &s
}

func (s *RelpServer) Start() error {
	return s.doStart(&s.statusMutex)
}

func (s *RelpServer) doStart(mu *sync.Mutex) (err error) {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	if s.status == FinalStopped {
		err = ServerDefinitelyStopped
		return
	}
	if s.status != Stopped && s.status != Waiting {
		err = ServerNotStopped
		return
	}

	err = s.initListeners()
	if err != nil {
		// net.OpError
		return
	}
	if !s.test {
		s.logger.Debug("trying to reach kafka")
		s.kafkaClient, err = s.Conf.GetKafkaClient()
		if err != nil {
			// sarama/kafka error
			s.resetListeners()
			return
		}
	}

	s.status = Started
	s.StatusChan <- Started

	s.wg.Add(1)
	go s.Listen()
	return
}

func (s *RelpServer) Stop() {
	s.doStop(false, false, &s.statusMutex)
}

func (s *RelpServer) FinalStop() {
	s.doStop(true, false, &s.statusMutex)
}

func (s *RelpServer) StopAndWait() {
	s.doStop(false, true, &s.statusMutex)
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
		}
		return
	}

	if s.status == Stopped || s.status == FinalStopped || s.status == Waiting {
		if s.status != Waiting && wait {
			s.status = Waiting
			s.StatusChan <- Waiting
		}
		return
	}

	s.resetListeners()
	// wait that all goroutines have ended
	s.wg.Wait()

	if s.kafkaClient != nil {
		s.kafkaClient.Close()
		s.kafkaClient = nil
	}

	if final {
		s.status = FinalStopped
		s.StatusChan <- FinalStopped
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

func (h RelpHandler) HandleConnection(conn net.Conn, i int) {
	// http://www.rsyslog.com/doc/relp.html
	s := h.Server
	s.AddConnection(conn)

	raw_messages_chan := make(chan *model.RelpRawMessage)
	parsed_messages_chan := make(chan *model.RelpParsedMessage)
	other_successes_chan := make(chan int)
	other_fails_chan := make(chan int)

	relpIsOpen := false

	var client string
	remote := conn.RemoteAddr()
	if remote != nil {
		client = strings.Split(remote.String(), ":")[0]
	}

	var local_port int
	local := conn.LocalAddr()
	if local != nil {
		s := strings.Split(local.String(), ":")
		local_port, _ = strconv.Atoi(s[len(s)-1])
	}

	logger := s.logger.New("remote", client, "local_port", local_port)
	logger.Info("New RELP client")

	// pull messages from raw_messages_chan and push them to parsed_messages_chan
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for m := range raw_messages_chan {
			p, err := model.Parse(m.Message, s.Conf.Syslog[i].Format)
			if err == nil {
				parsed_msg := model.RelpParsedMessage{
					Parsed: model.ParsedMessage{
						Fields:    p,
						Client:    m.Client,
						LocalPort: m.LocalPort,
					},
					Txnr: m.Txnr,
				}
				parsed_messages_chan <- &parsed_msg
			} else {
				logger.Info("Parsing error", "Message", m.Message)
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
	var err error

	if s.test {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			more_other_succs := true
			more_other_fails := true
			var other_txnr int

			for more_other_succs || more_other_fails {
				select {
				case other_txnr, more_other_succs = <-other_successes_chan:
					if more_other_succs {
						answer := fmt.Sprintf("%d rsp 6 200 OK\n", other_txnr)
						conn.Write([]byte(answer))
					}
				case other_txnr, more_other_fails = <-other_fails_chan:
					if more_other_fails {
						answer := fmt.Sprintf("%d rsp 6 500 KO\n", other_txnr)
						conn.Write([]byte(answer))
					}
				}
			}

		}()
	} else {
		producer, err = s.Conf.GetKafkaAsyncProducer()
		if err != nil {
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
			more_succs := true
			more_fails := true
			more_other_succs := true
			more_other_fails := true
			var other_txnr int
			var succ *sarama.ProducerMessage
			var fail *sarama.ProducerError
			successes := map[int]bool{}
			failures := map[int]bool{}
			last_committed_txnr := 0

			for more_succs || more_fails || more_other_succs || more_other_fails {
				select {
				case succ, more_succs = <-producer.Successes():
					if more_succs {
						// forward the ACK to rsyslog
						txnr := succ.Metadata.(int)
						successes[txnr] = true
					}
				case fail, more_fails = <-producer.Errors():
					if more_fails {
						// inform rsyslog that the message was not delivered
						txnr := fail.Msg.Metadata.(int)
						failures[txnr] = true
						logger.Info("NACK from Kafka", "error", fail.Error(), "txnr", txnr, "topic", fail.Msg.Topic)
						fatal = model.IsFatalKafkaError(fail.Err)
					}
				case other_txnr, more_other_succs = <-other_successes_chan:
					if more_other_succs {
						successes[other_txnr] = true
					}
				case other_txnr, more_other_fails = <-other_fails_chan:
					if more_other_fails {
						failures[other_txnr] = true
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
					} else if _, ok := failures[last_committed_txnr+1]; ok {
						last_committed_txnr++
						delete(failures, last_committed_txnr)
						answer := fmt.Sprintf("%d rsp 6 500 KO\n", last_committed_txnr)
						conn.Write([]byte(answer))
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
		for m := range parsed_messages_chan {
			// todo: optional filtering of parsed messages
			pkeyTmpl := s.Conf.Syslog[i].PartitionKeyTemplate
			topicTmpl := s.Conf.Syslog[i].TopicTemplate
			kafkaMsg, err := m.Parsed.ToKafka(pkeyTmpl, topicTmpl)
			if err != nil {
				s.logger.Warn("Error generating Kafka message", "error", err)
				other_fails_chan <- m.Txnr
				continue
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

	// RELP server
	scanner := bufio.NewScanner(conn)
	scanner.Split(RelpSplit)
	for {
		if scanner.Scan() {
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
					return
				}
				answer := fmt.Sprintf("%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, data)
				conn.Write([]byte(answer))
				relpIsOpen = true
				logger.Info("Received 'open' command")
			case "close":
				if !relpIsOpen {
					logger.Warn("Received close command before open")
					return
				}
				answer := fmt.Sprintf("%d rsp 0\n0 serverclose 0\n", txnr)
				conn.Write([]byte(answer))
				relpIsOpen = false
				logger.Info("Received 'close' command")
			case "syslog":
				if !relpIsOpen {
					logger.Warn("Received syslog command before open")
					return
				}
				raw := model.RelpRawMessage{
					Txnr: txnr,
					RawMessage: model.RawMessage{
						Message:   data,
						Client:    client,
						LocalPort: local_port,
					},
				}
				raw_messages_chan <- &raw
			default:
				logger.Warn("Unknown RELP command", "command", command)
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
