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

	"github.com/stephane-martin/relp2kafka/conf"
)

func main() {
	fmt.Println("vim-go")
}

type ServerStatus int

const (
	Stopped ServerStatus = iota
	Started
	Stopping
)

type RelpServer struct {
	conf      *conf.GlobalConfig
	listeners []net.Listener
	//stopChan    chan bool
	wg          sync.WaitGroup
	acceptsWg   sync.WaitGroup
	connections map[net.Conn]bool
	connMutex   sync.Mutex
	statusMutex sync.Mutex
	status      ServerStatus
	kafkaClient sarama.Client
	test        bool
}

func New(c *conf.GlobalConfig) *RelpServer {
	srv := RelpServer{conf: c}
	return &srv
}

func (s *RelpServer) Test() {
	s.test = true
}

func (s *RelpServer) init() error {
	if s == nil {
		return fmt.Errorf("Trying to init an void pointer!!!")
	}
	s.listeners = []net.Listener{}
	for _, syslogConf := range s.conf.Syslog {
		l, err := net.Listen("tcp", syslogConf.ListenAddr)
		if err != nil {
			// todo: close the previous listeners ?
			return err
		}
		s.listeners = append(s.listeners, l)
	}
	s.connections = map[net.Conn]bool{}
	//s.stopChan = make(chan bool, 1)
	s.status = Stopped
	return nil
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
					fmt.Println("closed listener")
					// can happen because we called the Stop() method
					return
				}
			default:
				// log the error and continue
				fmt.Println(accept_err)
			}
		} else if conn != nil {
			fmt.Println("new client")
			s.wg.Add(1)
			go s.handleConn(conn, i)
		}
	}
}

func (s *RelpServer) AcceptAll() {

	for i, _ := range s.listeners {
		s.acceptsWg.Add(1)
		s.wg.Add(1)
		go s.Accept(i)
	}
	s.acceptsWg.Wait()

	// the listeners are closed, we are now stopping
	fmt.Println("Stopping...")

	s.statusMutex.Lock()
	s.status = Stopping
	s.statusMutex.Unlock()

	// close the client connections
	s.CloseConnections()

	s.wg.Done()
	// wait that all goroutines have ended
	s.wg.Wait()

	s.statusMutex.Lock()
	s.status = Stopped
	s.statusMutex.Unlock()

	if !s.test {
		s.kafkaClient.Close()
	}
	fmt.Println("stopped")
}

func (s *RelpServer) Start() error {
	if s == nil {
		return fmt.Errorf("Start() on nil pointer!!!")
	}

	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != Stopped {
		return fmt.Errorf("Server is not stopped")
	}
	var err error
	if !s.test {
		s.kafkaClient, err = s.conf.GetKafkaClient()
		if err != nil {
			return err
		}
	}
	err = s.init()
	if err != nil {
		if !s.test {
			s.kafkaClient.Close()
		}
		return err
	}
	s.status = Started

	s.wg.Add(1)
	go s.AcceptAll()

	fmt.Println("started")
	return nil
}

func (s *RelpServer) Stop() {
	fmt.Println("Stop() method was called")
	if s == nil {
		return
	}

	s.statusMutex.Lock()
	if s.status != Started {
		s.statusMutex.Unlock()
		fmt.Println("Stop(): server is not started")
		return
	}
	s.status = Stopping
	s.statusMutex.Unlock()

	//s.stopChan <- true
	for _, l := range s.listeners {
		l.Close()
	}
	s.wg.Wait()
}

type RawMessage struct {
	Message   string
	Txnr      int
	Client    string
	LocalPort int
}

type ParsedMessage struct {
	Message   *SyslogMessage `json:"message"`
	Txnr      int            `json:"-"`
	Client    string         `json:"client"`
	LocalPort int            `json:"local_port,string"`
}

func (s *RelpServer) handleConn(conn net.Conn, i int) {
	// http://www.rsyslog.com/doc/relp.html
	s.AddConnection(conn)

	raw_messages_chan := make(chan *RawMessage)
	parsed_messages_chan := make(chan *ParsedMessage)

	// pull messages from raw_messages_chan and push them to parsed_messages_chan
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for m := range raw_messages_chan {
			p, err := Parse(m.Message, s.conf.Syslog[i].Format)
			if err == nil {
				parsed_msg := ParsedMessage{Message: p, Txnr: m.Txnr, Client: m.Client, LocalPort: m.LocalPort}
				parsed_messages_chan <- &parsed_msg
			}
		}
		close(parsed_messages_chan)
	}()

	defer func() {
		// closing raw_messages_chan causes parsed_messages_chan to be closed too, because of the goroutine just above
		close(raw_messages_chan)
		s.RemoveConnection(conn)
		fmt.Println("client connection closed")
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

	var producer sarama.AsyncProducer
	var err error
	if !s.test {
		producer, err = s.conf.GetKafkaAsyncProducer()
		if err != nil {
			fmt.Println("Can't get a kafka producer. Aborting handleConn.")
			return
		}
		// AsyncClose will eventually terminate the goroutine just below
		defer producer.AsyncClose()

		// listen for the ACKs coming from Kafka
		// this goroutine ends after producer.AsyncClose() is called
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			more_successes := true
			more_errors := true
			var one_success *sarama.ProducerMessage
			var one_fail *sarama.ProducerError
			successes := map[int]bool{}
			failures := map[int]bool{}
			last_committed_txnr := 0

			for more_successes || more_errors {
				select {
				case one_success, more_successes = <-producer.Successes():
					if more_successes {
						// forward the ACK to rsyslog
						txnr := one_success.Metadata.(int)
						successes[txnr] = true
					}
				case one_fail, more_errors = <-producer.Errors():
					if more_errors {
						// inform rsyslog that the message was not delivered
						txnr := one_fail.Msg.Metadata.(int)
						failures[txnr] = true

						switch one_fail.Err {
						case sarama.ErrOutOfBrokers:
							// the kafka cluster has gone away
							// there is no point to accept more messages from rsyslog
							// until the kafka cluster will be on again
							s.Stop()
						default:
							// log the error
						}
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
						delete(failures, last_committed_txnr)
						last_committed_txnr++
						answer := fmt.Sprintf("%d rsp 6 500 KO\n", last_committed_txnr)
						conn.Write([]byte(answer))
					} else {
						break
					}
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
				// todo: log err
				continue
			}
			pk_buf := bytes.Buffer{}
			err = s.conf.Syslog[i].PartitionKeyTemplate.Execute(&pk_buf, m)
			if err != nil {
				// todo
				continue
			}
			t_buf := bytes.Buffer{}
			err = s.conf.Syslog[i].TopicTemplate.Execute(&t_buf, m)
			if err != nil {
				// todo
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
					return
				}
				answer := fmt.Sprintf("%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, data)
				conn.Write([]byte(answer))
				relpIsOpen = true
			case "close":
				if !relpIsOpen {
					return
				}
				answer := fmt.Sprintf("%d rsp 0\n0 serverclose 0\n", txnr)
				conn.Write([]byte(answer))
				relpIsOpen = false
			case "syslog":
				if !relpIsOpen {
					return
				}
				raw := RawMessage{Message: data, Txnr: txnr, Client: client, LocalPort: local_port}
				raw_messages_chan <- &raw
			default:
				return
			}
		} else {
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
