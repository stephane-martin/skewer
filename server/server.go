package server

import (
	"net"
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/javascript"
	"github.com/stephane-martin/relp2kafka/model"
)

type Parser interface {
	Parse(m string, dont_parse_sd bool) (*model.SyslogMessage, error)
}

type StreamHandler interface {
	HandleConnection(conn net.Conn, i int)
}

type PacketHandler interface {
	HandleConnection(conn net.PacketConn, i int)
}

type Connection interface {
	Close() error
}

type Server struct {
	Conf            conf.GConfig
	logger          log15.Logger
	unixSocketPaths []string
	test            bool
	wg              *sync.WaitGroup
	protocol        string
	parsersEnv      *javascript.Environment
	connections     map[Connection]bool
	connMutex       *sync.Mutex
	statusMutex     *sync.Mutex
}

func (s *Server) init() {
	s.wg = &sync.WaitGroup{}
	s.unixSocketPaths = []string{}
	s.connMutex = &sync.Mutex{}
	s.connections = map[Connection]bool{}
	s.statusMutex = &sync.Mutex{}
}

type StreamServer struct {
	Server
	tcpListeners  map[int]*net.TCPListener
	unixListeners map[int]*net.UnixListener
	acceptsWg     *sync.WaitGroup
	handler       StreamHandler
}

func (s *StreamServer) init() {
	s.Server.init()
	s.tcpListeners = map[int]*net.TCPListener{}
	s.unixListeners = map[int]*net.UnixListener{}
	s.acceptsWg = &sync.WaitGroup{}
}

func (s *Server) initParsers() {
	s.parsersEnv = javascript.New("", "", nil, "", nil, s.logger)
	for _, parserConf := range s.Conf.Parsers {
		err := s.parsersEnv.AddParser(parserConf.Name, parserConf.Func)
		if err != nil {
			s.logger.Warn("Error initializing parser", "name", parserConf.Name, "error", err)
		}
	}
}

func (s *Server) GetParser(parserName string) Parser {
	switch parserName {
	case "rfc5424", "rfc3164", "json", "auto":
		return model.GetParser(parserName)
	default:
		return s.parsersEnv.GetParser(parserName)
	}
}

func (s *StreamServer) initTCPListeners() int {
	nb := 0
	s.connections = map[Connection]bool{}
	s.tcpListeners = map[int]*net.TCPListener{}
	s.unixListeners = map[int]*net.UnixListener{}
	for i, syslogConf := range s.Conf.Syslog {
		if syslogConf.Protocol != s.protocol {
			continue
		}
		if len(syslogConf.UnixSocketPath) > 0 {
			unixAddr, err := net.ResolveUnixAddr("unix", syslogConf.UnixSocketPath)
			if err != nil {
				s.logger.Warn("Error resolving Unix socket address", "path", syslogConf.UnixSocketPath, "error", err)
				continue
			}
			l, err := net.ListenUnix("unix", unixAddr)
			if err == nil {
				s.logger.Info("Listener", "protocol", s.protocol, "path", syslogConf.UnixSocketPath, "format", syslogConf.Format)
				nb++
				s.unixListeners[i] = l
			} else {
				s.logger.Error("Error listening on stream unix socket", "path", syslogConf.UnixSocketPath, "error", err)
				continue
			}
		} else {
			tcpAddr, err := net.ResolveTCPAddr("tcp", syslogConf.ListenAddr)
			if err != nil {
				s.logger.Warn("Error resolving TCP address", "address", syslogConf.ListenAddr, "error", err)
				continue
			}
			l, err := net.ListenTCP("tcp", tcpAddr)
			if err == nil {
				s.logger.Info("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
				nb++
				s.tcpListeners[i] = l
			} else {
				s.logger.Error("Error listening on stream (TCP or RELP)", "listen_addr", syslogConf.ListenAddr, "error", err)
				continue
			}
		}
	}
	return nb
}

func (s *StreamServer) resetTCPListeners() {
	for _, l := range s.tcpListeners {
		l.Close()
	}
	for _, l := range s.unixListeners {
		l.Close()
	}
	s.tcpListeners = map[int]*net.TCPListener{}
}

func (s *Server) SetTest() {
	s.test = true
}

func (s *Server) AddConnection(conn Connection) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.connections[conn] = true
}

func (s *Server) RemoveConnection(conn Connection) {
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
	for _, path := range s.unixSocketPaths {
		os.Remove(path)
	}
	s.connections = map[Connection]bool{}
	s.unixSocketPaths = []string{}
}

func (s *StreamServer) handleConnection(conn net.Conn, i int) {
	s.handler.HandleConnection(conn, i)
}

func (s *StreamServer) AcceptUnix(i int) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, accept_err := s.unixListeners[i].AcceptUnix()
		if accept_err != nil {
			s.logger.Info("AcceptUnix() error", "error", accept_err)
			switch accept_err.(type) {
			case *net.OpError:
				return
			default:
				// continue
			}
		} else if conn != nil {
			go s.handleConnection(conn, i)
		}
	}

}

func (s *StreamServer) AcceptTCP(i int) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, accept_err := s.tcpListeners[i].AcceptTCP()
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
			go s.handleConnection(conn, i)
		}
	}
}

func (s *StreamServer) Listen() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for i, _ := range s.tcpListeners {
			s.acceptsWg.Add(1)
			s.wg.Add(1)
			go s.AcceptTCP(i)
		}
		for i, _ := range s.unixListeners {
			s.acceptsWg.Add(1)
			s.wg.Add(1)
			go s.AcceptUnix(i)
		}
		// wait until the listeners stop and return
		s.acceptsWg.Wait()
		// close the client connections
		s.CloseConnections()
	}()
}
