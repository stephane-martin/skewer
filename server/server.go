package server

import (
	"net"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/store"
	sarama "gopkg.in/Shopify/sarama.v1"
)

type StreamHandler interface {
	HandleConnection(conn net.Conn, i int)
}

type PacketHandler interface {
	HandleConnection(conn net.PacketConn, i int)
}

type Server struct {
	Conf              conf.GConfig
	listeners         map[int]net.Listener
	logger            log15.Logger
	connections       map[net.Conn]bool
	packetConnections map[net.PacketConn]bool
	connMutex         *sync.Mutex
	kafkaClient       sarama.Client
	test              bool
	wg                *sync.WaitGroup
	acceptsWg         *sync.WaitGroup
	shandler          StreamHandler
	phandler          PacketHandler
	protocol          string
	stream            bool
}

func (s *Server) init() {
	s.connMutex = &sync.Mutex{}
	s.wg = &sync.WaitGroup{}
	s.acceptsWg = &sync.WaitGroup{}
}

type StoreServer struct {
	Server
	store          *store.MessageStore
	storeToKafkaWg *sync.WaitGroup
	producer       sarama.AsyncProducer
}

func (s *StoreServer) init() {
	s.Server.init()
	s.storeToKafkaWg = &sync.WaitGroup{}
}

func (s *Server) initListeners() int {
	nb := 0
	s.connections = map[net.Conn]bool{}
	s.listeners = map[int]net.Listener{}
	for i, syslogConf := range s.Conf.Syslog {
		if syslogConf.Protocol != s.protocol {
			continue
		}
		s.logger.Info("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
		l, err := net.Listen("tcp", syslogConf.ListenAddr)
		if err == nil {
			nb++
			s.listeners[i] = l
		} else {
			s.logger.Error("Error listening on stream (TCP or RELP)", "listen_addr", syslogConf.ListenAddr, "error", err)
		}
	}
	return nb
}

func (s *Server) resetListeners() {
	if s.stream {
		for _, l := range s.listeners {
			l.Close()
		}
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

func (s *Server) AddPacketConnection(conn net.PacketConn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.packetConnections[conn] = true
}

func (s *Server) RemoveConnection(conn net.Conn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	if _, ok := s.connections[conn]; ok {
		conn.Close()
		delete(s.connections, conn)
	}
}

func (s *Server) RemovePacketConnection(conn net.PacketConn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	if _, ok := s.packetConnections[conn]; ok {
		conn.Close()
		delete(s.packetConnections, conn)
	}
}

func (s *Server) CloseConnections() {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	for conn, _ := range s.connections {
		conn.Close()
	}
	for conn, _ := range s.packetConnections {
		conn.Close()
	}
	s.connections = map[net.Conn]bool{}
	s.packetConnections = map[net.PacketConn]bool{}

}

func (s *Server) handleStreamConnection(conn net.Conn, i int) {
	s.shandler.HandleConnection(conn, i)
}

func (s *Server) handlePacketConnection(conn net.PacketConn, i int) {
	s.phandler.HandleConnection(conn, i)
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
			go s.handleStreamConnection(conn, i)
		}
	}
}

func (s *Server) Listen() {
	if s.stream {
		for i, _ := range s.listeners {
			s.acceptsWg.Add(1)
			s.wg.Add(1)
			go s.Accept(i)
		}
		// wait until the listeners stop and return
		s.acceptsWg.Wait()
		// close the client connections
		s.CloseConnections()
	}
	s.wg.Done()

}

func (s *Server) ListenPacket() int {
	nb := 0
	for i, syslogConf := range s.Conf.Syslog {
		if syslogConf.Protocol != s.protocol {
			continue
		}
		conn, err := net.ListenPacket("udp", syslogConf.ListenAddr)
		if err != nil {
			s.logger.Warn("Error listening on UDP", "addr", syslogConf.ListenAddr)
		} else if conn != nil {
			s.logger.Info("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
			nb++
			s.wg.Add(1)
			go s.handlePacketConnection(conn, i)
		}
	}
	return nb
}
