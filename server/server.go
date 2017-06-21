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
	HandleConnection(conn *net.TCPConn, i int)
}

type PacketHandler interface {
	HandleConnection(conn net.PacketConn, i int)
}

type Server struct {
	Conf              conf.GConfig
	listeners         map[int]*net.TCPListener
	logger            log15.Logger
	connections       map[*net.TCPConn]bool
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

func (s *Server) initTCPListeners() int {
	nb := 0
	s.connections = map[*net.TCPConn]bool{}
	s.listeners = map[int]*net.TCPListener{}
	for i, syslogConf := range s.Conf.Syslog {
		if syslogConf.Protocol != s.protocol {
			continue
		}
		s.logger.Info("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
		tcpAddr, err := net.ResolveTCPAddr("tcp", syslogConf.ListenAddr)
		if err != nil {
			s.logger.Warn("Error resolving TCP address", "address", syslogConf.ListenAddr)
			continue
		}
		l, err := net.ListenTCP("tcp", tcpAddr)
		if err == nil {
			nb++
			s.listeners[i] = l
		} else {
			s.logger.Error("Error listening on stream (TCP or RELP)", "listen_addr", syslogConf.ListenAddr, "error", err)
			continue
		}
	}
	return nb
}

func (s *Server) resetTCPListeners() {
	if s.stream {
		for _, l := range s.listeners {
			l.Close()
		}
	}
	s.listeners = map[int]*net.TCPListener{}
}

func (s *Server) SetTest() {
	s.test = true
}

func (s *Server) AddTCPConnection(conn *net.TCPConn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.connections[conn] = true
}

func (s *Server) AddPacketConnection(conn net.PacketConn) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.packetConnections[conn] = true
}

func (s *Server) RemoveTCPConnection(conn *net.TCPConn) {
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
	s.connections = map[*net.TCPConn]bool{}
	s.packetConnections = map[net.PacketConn]bool{}

}

func (s *Server) handleStreamConnection(conn *net.TCPConn, i int) {
	s.shandler.HandleConnection(conn, i)
}

func (s *Server) handlePacketConnection(conn net.PacketConn, i int) {
	s.phandler.HandleConnection(conn, i)
}

func (s *Server) AcceptTCP(i int) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, accept_err := s.listeners[i].AcceptTCP()
		if accept_err != nil {
			s.logger.Info("AcceptTCP() error", "error", accept_err)
			switch accept_err.(type) {
			case *net.OpError:
				return
			default:
				// continue
			}
		} else if conn != nil {
			if s.Conf.Syslog[i].KeepAlive {
				err := conn.SetKeepAlive(true)
				if err == nil {
					err := conn.SetKeepAlivePeriod(s.Conf.Syslog[i].KeepAlivePeriod)
					if err != nil {
						s.logger.Warn("Error setting keepalive period", "addr", s.Conf.Syslog[i].ListenAddr, "period", s.Conf.Syslog[i].KeepAlivePeriod)
					}
				} else {
					s.logger.Warn("Error setting keepalive", "addr", s.Conf.Syslog[i].ListenAddr)
				}

			} else {
				err := conn.SetKeepAlive(false)
				if err != nil {
					s.logger.Warn("Error disabling keepalive", "addr", s.Conf.Syslog[i].ListenAddr)
				}
			}
			err := conn.SetNoDelay(true)
			if err != nil {
				s.logger.Warn("Error setting TCP NODELAY", "addr", s.Conf.Syslog[i].ListenAddr)
			}
			err = conn.SetLinger(-1)
			if err != nil {
				s.logger.Warn("Error setting TCP LINGER", "addr", s.Conf.Syslog[i].ListenAddr)
			}
			s.wg.Add(1)
			go s.handleStreamConnection(conn, i)
		}
	}
}

func (s *Server) ListenTCP() {
	if s.stream {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for i, _ := range s.listeners {
				s.acceptsWg.Add(1)
				s.wg.Add(1)
				go s.AcceptTCP(i)
			}
			// wait until the listeners stop and return
			s.acceptsWg.Wait()
			// close the client connections
			s.CloseConnections()
		}()
	}
}

func (s *Server) ListenPacket() int {
	nb := 0
	for i, syslogConf := range s.Conf.Syslog {
		switch syslogConf.Protocol {
		case "udp":
			conn, err := net.ListenPacket("udp", syslogConf.ListenAddr)
			if err != nil {
				s.logger.Warn("Error listening on UDP", "addr", syslogConf.ListenAddr)
			} else if conn != nil {
				s.logger.Info("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
				nb++
				s.wg.Add(1)
				go s.handlePacketConnection(conn, i)
			}

		case "unixgram":
			conn, err := net.ListenPacket("unixgram", syslogConf.UnixSocketPath)
			if err != nil {
				s.logger.Warn("Error listening on datagram unix socket", "path", syslogConf.UnixSocketPath)
			} else if conn != nil {
				s.logger.Info("Listener", "protocol", s.protocol, "path", syslogConf.UnixSocketPath, "format", syslogConf.Format)
				nb++
				s.wg.Add(1)
				go s.handlePacketConnection(conn, i)
			}
		default:
		}

	}
	return nb
}
