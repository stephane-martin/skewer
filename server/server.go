package server

import (
	"net"
	"sync"
	"unicode/utf8"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/conf"
	sarama "gopkg.in/Shopify/sarama.v1"
)

type Handler interface {
	HandleConnection(conn net.Conn, i int)
}

type Server struct {
	Conf        *conf.GlobalConfig
	listeners   map[int]net.Listener
	logger      log15.Logger
	connections map[net.Conn]bool
	connMutex   sync.Mutex
	kafkaClient sarama.Client
	test        bool
	wg          sync.WaitGroup
	acceptsWg   sync.WaitGroup
	handler     Handler
	protocol    string
}

func (s *Server) initListeners() error {
	s.logger.Debug("initListeners")
	s.listeners = map[int]net.Listener{}
	for i, syslogConf := range s.Conf.Syslog {
		if syslogConf.Protocol != s.protocol {
			continue
		}
		s.logger.Info("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
		l, err := net.Listen("tcp", syslogConf.ListenAddr)
		if err != nil {
			for _, pl := range s.listeners {
				pl.Close()
			}
			s.listeners = map[int]net.Listener{}
			// returns a net.OpError
			return err
		}
		s.listeners[i] = l
	}
	s.connections = map[net.Conn]bool{}
	return nil
}

func (s *Server) resetListeners() {
	s.logger.Debug("resetListeners")
	for _, l := range s.listeners {
		l.Close()
	}
	s.listeners = map[int]net.Listener{}
}

func (s *Server) SetTest() {
	s.test = true
}

func (s *Server) AddConnection(conn net.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.connections[conn] = true
}

func (s *Server) RemoveConnection(conn net.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	if _, ok := s.connections[conn]; ok {
		conn.Close()
		delete(s.connections, conn)
	}
}

func (s *Server) CloseConnections() {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	for conn, _ := range s.connections {
		conn.Close()
	}
	s.connections = map[net.Conn]bool{}

}

func (s *Server) handleConnection(conn net.Conn, i int) {
	s.handler.HandleConnection(conn, i)
}

func (s *Server) Accept(i int) {
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
				s.logger.Error("Accept() error", "error", accept_err)
			}
		} else if conn != nil {
			s.wg.Add(1)
			go s.handleConnection(conn, i)
		}
	}
}

func (s *Server) Listen() {
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

func TopicNameIsValid(name string) bool {
	if len(name) == 0 {
		return false
	}
	if len(name) > 249 {
		return false
	}
	if !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		if !validRune(r) {
			return false
		}
	}
	return true
}

func validRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r == '.' {
		return true
	}
	if r == '_' {
		return true
	}
	if r == '-' {
		return true
	}
	return false
}
