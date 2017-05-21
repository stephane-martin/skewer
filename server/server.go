package server

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

func main() {
	fmt.Println("vim-go")
}

type Server struct {
	listenAddr  string
	listener    net.Listener
	stopChan    chan bool
	wg          sync.WaitGroup
	connections map[net.Conn]bool
	mu          sync.Mutex
}

func New(listen_addr string) (s *Server, err error) {
	srv := Server{listenAddr: listen_addr}
	s = &srv
	s.stopChan = make(chan bool)
	var l net.Listener
	l, err = net.Listen("tcp", s.listenAddr)
	if err != nil {
		s = nil
		return
	}
	s.listener = l
	s.connections = map[net.Conn]bool{}
	return
}

func (s *Server) AddConnection(conn net.Conn) {
	s.mu.Lock()
	s.connections[conn] = true
	s.mu.Unlock()
}

func (s *Server) RemoveConnection(conn net.Conn) {
	s.mu.Lock()
	if _, ok := s.connections[conn]; ok {
		conn.Close()
		delete(s.connections, conn)
	}
	s.mu.Unlock()
}

func (s *Server) CloseConnections() {
	s.mu.Lock()
	for conn, _ := range s.connections {
		conn.Close()
		delete(s.connections, conn)
	}
	s.mu.Unlock()
}

func (s *Server) Start() {
	if s == nil {
		return
	}
	go func() {
		s.wg.Add(1)
		defer func() { s.wg.Done() }()
		for {
			select {
			case <-s.stopChan:
				return
			default:
			}
			conn, accept_err := s.listener.Accept()
			if accept_err != nil {
				switch t := accept_err.(type) {
				case *net.OpError:
					if t.Err.Error() == "use of closed network connection" {
						return
					}
				default:
				}
				// log the error and continue
				fmt.Println(accept_err)
			} else if conn != nil {
				fmt.Println("new client")
				go s.handleConn(conn)
			}
		}
	}()
}

type RawMessage struct {
	Message string
	Txnr int
	Client string
	LocalPort int
}

type ParsedMessage struct {
	Message *SyslogMessage
	Txnr int
	Client string
	LocalPort int
}

func (s *Server) handleConn(conn net.Conn) {
	// http://www.rsyslog.com/doc/relp.html
	s.wg.Add(1)
	s.AddConnection(conn)

	raw_messages_chan := make(chan *RawMessage)
	parsed_messages_chan := make(chan *ParsedMessage)

	defer func() {
		close(raw_messages_chan)
		s.RemoveConnection(conn)
		s.wg.Done()
		fmt.Println("client connection closed")
	}()

	opened := false

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

	parse_messages_func := func() {
		s.wg.Add(1)
		for m := range raw_messages_chan {
			p, err := Parse(m.Message)
			if err == nil {
				parsed_msg := ParsedMessage{Message: p, Txnr: m.Txnr, Client: m.Client, LocalPort: m.LocalPort}	
				parsed_messages_chan <- &parsed_msg
			}
		}
		close(parsed_messages_chan)
		s.wg.Done()
	}

	push_parsed_messages_func := func() {
		s.wg.Add(1)
		for m := range parsed_messages_chan {
			fmt.Println(m.Client)
			fmt.Println(m.LocalPort)
			fmt.Println()
		}
		s.wg.Done()
	}

	go push_parsed_messages_func()
	go parse_messages_func()

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
			answer := ""
			switch command {
			case "open":
				if opened {
					return
				}
				answer = fmt.Sprintf("%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, data)
				opened = true
			case "close":
				if !opened {
					return
				}
				answer = fmt.Sprintf("%d rsp 0\n0 serverclose 0\n", txnr)
				opened = false
			case "syslog":
				if !opened {
					return
				}
				answer = fmt.Sprintf("%d rsp 6 200 OK\n", txnr)
				raw := RawMessage{Message: data, Txnr: txnr, Client: client, LocalPort: local_port}
				raw_messages_chan <- &raw
			default:
				return
			}
			conn.Write([]byte(answer))
		} else {
			return
		}
	}
}

func (s *Server) Stop() {
	if s != nil {
		close(s.stopChan)
		s.listener.Close()
		s.CloseConnections()
		s.wg.Wait()
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

	txnr, err := strconv.Atoi(txnr_s)
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
