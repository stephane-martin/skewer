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
	conf        *conf.GlobalConfig
	listener    net.Listener
	stopChan    chan bool
	wg          sync.WaitGroup
	connections map[net.Conn]bool
	connMutex   sync.Mutex
	statusMutex sync.Mutex
	status      ServerStatus
	kafkaClient sarama.Client
}

func New(c *conf.GlobalConfig) *RelpServer {
	srv := RelpServer{conf: c}
	return &srv
}

func (s *RelpServer) init() error {
	if s == nil {
		return fmt.Errorf("Trying to init an void pointer!!!")
	}
	l, err := net.Listen("tcp", s.conf.Syslog.ListenAddr)
	if err != nil {
		return err
	}
	s.listener = l
	s.connections = map[net.Conn]bool{}
	s.stopChan = make(chan bool, 1)
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
	/*
		s.kafkaClient, err = s.conf.GetKafkaClient()
		if err != nil {
			return err
		}
	*/
	err = s.init()
	if err != nil {
		//s.kafkaClient.Close()
		return err
	}
	s.status = Started

	go func() {
		s.wg.Add(1)
		defer func() {
			fmt.Println("Stopping...")
			s.statusMutex.Lock()
			s.status = Stopping
			s.statusMutex.Unlock()

			s.listener.Close()
			s.CloseConnections()

			s.wg.Done()
			s.wg.Wait()

			s.statusMutex.Lock()
			s.status = Stopped
			s.statusMutex.Unlock()
			close(s.stopChan)
			//s.kafkaClient.Close()
			fmt.Println("stopped")

		}()
		for {
			select {
			case <-s.stopChan:
				fmt.Println("notified by stopChan!")
				return
			default:
			}
			conn, accept_err := s.listener.Accept()
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
				go s.handleConn(conn)
			}
		}
	}()
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

	s.stopChan <- true
	s.listener.Close()

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

// implement the sarama.Encoder interface
func (m *ParsedMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}

func (m *ParsedMessage) Length() int {
	return 0
}

func (s *RelpServer) handleConn(conn net.Conn) {
	// http://www.rsyslog.com/doc/relp.html
	s.wg.Add(1)
	s.AddConnection(conn)

	raw_messages_chan := make(chan *RawMessage)
	parsed_messages_chan := make(chan *ParsedMessage)

	// pull messages from raw_messages_chan and push them to parsed_messages_chan
	go func() {
		s.wg.Add(1)
		defer s.wg.Done()
		for m := range raw_messages_chan {
			p, err := Parse(m.Message)
			if err == nil {
				parsed_msg := ParsedMessage{Message: p, Txnr: m.Txnr, Client: m.Client, LocalPort: m.LocalPort}
				parsed_messages_chan <- &parsed_msg
			}
		}
		close(parsed_messages_chan)
	}()

	defer func() {
		// closing raw_messages_chan causes parsed_messages_chan to be closed too
		close(raw_messages_chan)
		s.RemoveConnection(conn)
		s.wg.Done()
		fmt.Println("client connection closed")
	}()

	relpIsOpen := false

	var client string
	remote := conn.RemoteAddr()
	if remote != nil {
		client = remote.String()
	}

	var local_port int
	local := conn.LocalAddr()
	if local != nil {
		s := strings.Split(local.String(), ":")
		local_port, _ = strconv.Atoi(s[len(s)-1])
	}

	/*
		producer, err := s.conf.GetKafkaAsyncProducer()
		if err != nil {
			fmt.Println("Can't get a kafka producer. Aborting handleConn.")
			return
		}
	*/

	// listen to ACK from Kafka
	/*
		go func() {
			for success := range producer.Successes() {
				fmt.Println(success)
			}
		}()
	*/

	// listen to errors from Kafka

	// push parsed messages to Kafka
	push_parsed_messages_func := func() {
		s.wg.Add(1)
		defer s.wg.Done()
		for m := range parsed_messages_chan {
			// push m to Kafka

			fmt.Println(m.Client)
			fmt.Println(m.LocalPort)
			e, _ := m.Encode()
			fmt.Println(e)
		}
	}

	go push_parsed_messages_func()

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
				//answer = fmt.Sprintf("%d rsp 6 200 OK\n", txnr)
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
