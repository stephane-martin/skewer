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
	s.connections[conn] = false
	s.mu.Unlock()
}

func (s *Server) CloseConnections() {
	s.mu.Lock()
	for conn, ok := range s.connections {
		if ok {
			conn.Close()
		}
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
				go s.handleConn(conn)
			}
		}
	}()
}

func (s *Server) handleConn(conn net.Conn) {
	s.wg.Add(1)
	s.AddConnection(conn)

	defer func() {
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

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
				data = strings.Trim(splits[3], " \n")
			}
			answer := ""
			switch command {
			case "open":
				answer = fmt.Sprintf("%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, data)
				conn.Write([]byte(answer))
			case "close":
				answer = fmt.Sprintf("%d rsp 0\n0 serverclose 0\n", txnr)
				conn.Write([]byte(answer))
			case "syslog":
				answer = fmt.Sprintf("%d rsp 6 200 OK\n", txnr)
			default:
				return
			}
			fmt.Println("Got line!")
			fmt.Println(line)
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
	return r == ' ' || r == '\n'
}

func RelpSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	trimmed_data := bytes.TrimLeft(data, " \n")
	splits := bytes.FieldsFunc(trimmed_data, splitSpaceOrLF)
	if len(splits) >= 4 {
		txnr_s := string(splits[0])
		command := string(splits[1])
		datalen_s := string(splits[2])
		_, err = strconv.Atoi(txnr_s)
		if err != nil {
			return 0, nil, err
		}
		datalen, err := strconv.Atoi(datalen_s)
		if err != nil {
			return 0, nil, err
		}
		token_s := txnr_s + " " + command + " " + datalen_s
		advance := len(data) - len(trimmed_data) + len(token_s) + 1
		if datalen == 0 {
			return advance, []byte(token_s), nil
		}
		advance += datalen + 1
		if len(data) >= advance {
			token := bytes.Trim(data[:advance], " \n")
			return advance, token, nil
		}
	}

	// Request more data
	return 0, nil, nil
}
