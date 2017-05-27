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
	confDirName string
	conf        *conf.GlobalConfig
	listeners   []net.Listener
	wg          sync.WaitGroup
	acceptsWg   sync.WaitGroup
	connections map[net.Conn]bool
	connMutex   sync.Mutex
	statusMutex sync.Mutex
	status      RelpServerStatus
	StatusChan  chan RelpServerStatus
	kafkaClient sarama.Client
	test        bool
	logger      log15.Logger
}

func New(dirname string, logger log15.Logger) *RelpServer {
	s := RelpServer{}
	s.confDirName = dirname
	s.StatusChan = make(chan RelpServerStatus, 1)
	s.status = Stopped
	s.listeners = []net.Listener{}
	s.connections = map[net.Conn]bool{}
	s.logger = logger
	return &s
}

func (s *RelpServer) SetTest() {
	s.test = true
}

func (s *RelpServer) initConf() error {
	c, err := conf.Load(s.confDirName)
	if err != nil {
		// returns a conf.ConfigurationError
		return err
	}
	s.conf = c
	return nil
}

func (s *RelpServer) initListeners() error {
	s.listeners = []net.Listener{}
	for _, syslogConf := range s.conf.Syslog {
		s.logger.Info("Listener", "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
		l, err := net.Listen("tcp", syslogConf.ListenAddr)
		if err != nil {
			for _, pl := range s.listeners {
				pl.Close()
			}
			s.listeners = []net.Listener{}
			// returns a net.OpError
			return err
		}
		s.listeners = append(s.listeners, l)
	}
	s.connections = map[net.Conn]bool{}
	return nil
}

func (s *RelpServer) resetListeners() {
	for _, l := range s.listeners {
		l.Close()
	}
	s.listeners = []net.Listener{}
}

func (s *RelpServer) AddConnection(conn net.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.connections[conn] = true
}

func (s *RelpServer) RemoveConnection(conn net.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	if _, ok := s.connections[conn]; ok {
		conn.Close()
		delete(s.connections, conn)
	}
}

func (s *RelpServer) CloseConnections() {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	for conn, _ := range s.connections {
		conn.Close()
		delete(s.connections, conn)
	}
}

func (s *RelpServer) Accept(i int) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, accept_err := s.listeners[i].Accept()
		if accept_err != nil {
			switch t := accept_err.(type) {
			case *net.OpError:
				if t.Err.Error() == "use of closed network connection" {
					// can happen because we called the Stop() method
					return
				}
			default:
				// log the error and continue
				fmt.Println(accept_err)
			}
		} else if conn != nil {
			s.wg.Add(1)
			go s.handleRelpConnection(conn, i)
		}
	}
}

func (s *RelpServer) Listen() {
	for i, _ := range s.listeners {
		s.acceptsWg.Add(1)
		s.wg.Add(1)
		go s.Accept(i)
	}
	// wait until the listeners stop and return
	s.acceptsWg.Wait()
	// close the client connections
	s.CloseConnections()
	s.wg.Done()
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

	err = s.initConf()
	if err != nil {
		// ConfigurationError
		return
	}
	err = s.initListeners()
	if err != nil {
		// net.OpError
		return
	}
	if !s.test {
		s.logger.Debug("trying to reach kafka")
		s.kafkaClient, err = s.conf.GetKafkaClient()
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

type RawMessage struct {
	Message   string
	Txnr      int
	Client    string
	LocalPort int
}

type ParsedMessage struct {
	Message   *model.SyslogMessage `json:"message"`
	Txnr      int                  `json:"-"`
	Client    string               `json:"client"`
	LocalPort int                  `json:"local_port,string"`
}

func isFatalKafkaError(e error) bool {
	switch e {
	case sarama.ErrOutOfBrokers,
		sarama.ErrUnknown,
		sarama.ErrUnknownTopicOrPartition,
		sarama.ErrReplicaNotAvailable,
		sarama.ErrLeaderNotAvailable,
		sarama.ErrNotLeaderForPartition,
		sarama.ErrNetworkException,
		sarama.ErrOffsetsLoadInProgress,
		sarama.ErrInvalidTopic,
		sarama.ErrNotEnoughReplicas,
		sarama.ErrNotEnoughReplicasAfterAppend,
		sarama.ErrTopicAuthorizationFailed,
		sarama.ErrGroupAuthorizationFailed,
		sarama.ErrClusterAuthorizationFailed,
		sarama.ErrUnsupportedVersion,
		sarama.ErrUnsupportedForMessageFormat,
		sarama.ErrPolicyViolation:
		return true
	default:
		return false
	}
}

func (s *RelpServer) handleRelpConnection(conn net.Conn, i int) {
	// http://www.rsyslog.com/doc/relp.html
	s.AddConnection(conn)

	raw_messages_chan := make(chan *RawMessage)
	parsed_messages_chan := make(chan *ParsedMessage)

	// pull messages from raw_messages_chan and push them to parsed_messages_chan
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for m := range raw_messages_chan {
			p, err := model.Parse(m.Message, s.conf.Syslog[i].Format)
			if err == nil {
				parsed_msg := ParsedMessage{Message: p, Txnr: m.Txnr, Client: m.Client, LocalPort: m.LocalPort}
				parsed_messages_chan <- &parsed_msg
			} else {
				// log the parsing error
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
	logger.Info("New client")

	var producer sarama.AsyncProducer
	var err error
	if !s.test {
		producer, err = s.conf.GetKafkaAsyncProducer()
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
			var succ *sarama.ProducerMessage
			var fail *sarama.ProducerError
			successes := map[int]bool{}
			failures := map[int]bool{}
			last_committed_txnr := 0

			for more_succs || more_fails {
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
						fatal = isFatalKafkaError(fail.Err)
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
		defer s.wg.Done()
		for m := range parsed_messages_chan {
			// todo: optional filtering of parsed messages
			value, err := json.Marshal(m)
			if err != nil {
				logger.Warn("Error marshaling a message to JSON", "error", err)
				continue
			}
			pk_buf := bytes.Buffer{}
			err = s.conf.Syslog[i].PartitionKeyTemplate.Execute(&pk_buf, m)
			if err != nil {
				logger.Warn("Error generating the partition hash key", "error", err)
				continue
			}
			t_buf := bytes.Buffer{}
			err = s.conf.Syslog[i].TopicTemplate.Execute(&t_buf, m)
			if err != nil {
				logger.Warn("Error generating the topic", "error", err)
				continue
			}

			if s.test {
				fmt.Println(string(value))
				answer := fmt.Sprintf("%d rsp 6 200 OK\n", m.Txnr)
				conn.Write([]byte(answer))
			} else {
				// todo: sanitize topic so that it will be accepted by kafka
				pm := sarama.ProducerMessage{
					Key:       sarama.ByteEncoder(pk_buf.Bytes()),
					Value:     sarama.ByteEncoder(value),
					Topic:     t_buf.String(),
					Timestamp: *m.Message.TimeReported,
					Metadata:  m.Txnr,
				}
				producer.Input() <- &pm
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
			case "close":
				if !relpIsOpen {
					logger.Warn("Received close command before open")
					return
				}
				answer := fmt.Sprintf("%d rsp 0\n0 serverclose 0\n", txnr)
				conn.Write([]byte(answer))
				relpIsOpen = false
			case "syslog":
				if !relpIsOpen {
					logger.Warn("Received syslog command before open")
					return
				}
				raw := RawMessage{Message: data, Txnr: txnr, Client: client, LocalPort: local_port}
				raw_messages_chan <- &raw
			default:
				logger.Warn("Unknown RELP command", "command", command)
				return
			}
		} else {
			logger.Info("Scanning the client stream has ended", "error", scanner.Err())
			return
		}
	}
}

func splitSpaceOrLF(r rune) bool {
	return r == ' ' || r == '\n' || r == '\r'
}

// RelpSplit is used to extract RELP lines from the incoming TCP stream
func RelpSplit(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	trimmed_data := bytes.TrimLeft(data, " \r\n")
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
