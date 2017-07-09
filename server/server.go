package server

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/consul"
	"github.com/stephane-martin/relp2kafka/javascript"
	"github.com/stephane-martin/relp2kafka/model"
)

type Parser interface {
	Parse(m string, dont_parse_sd bool) (*model.SyslogMessage, error)
}

type StreamHandler interface {
	HandleConnection(conn net.Conn, config conf.SyslogConfig)
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

type TCPListenerConf struct {
	Listener *net.TCPListener
	Conf     conf.SyslogConfig
}

type UnixListenerConf struct {
	Listener *net.UnixListener
	Conf     conf.SyslogConfig
}

type StreamServer struct {
	Server
	tcpListeners  []*TCPListenerConf
	unixListeners []*UnixListenerConf
	acceptsWg     *sync.WaitGroup
	handler       StreamHandler
}

func (s *StreamServer) init() {
	s.Server.init()
	s.tcpListeners = []*TCPListenerConf{}
	s.unixListeners = []*UnixListenerConf{}
	s.acceptsWg = &sync.WaitGroup{}
}

type ParsersEnv struct {
	jsenv javascript.ParsersEnvironment
}

func (s *Server) NewParsersEnv() *ParsersEnv {
	p := javascript.NewParsersEnvironment(s.logger)
	for _, parserConf := range s.Conf.Parsers {
		err := p.AddParser(parserConf.Name, parserConf.Func)
		if err != nil {
			s.logger.Warn("Error initializing parser", "name", parserConf.Name, "error", err)
		}
	}
	return &ParsersEnv{p}
}

func (e *ParsersEnv) GetParser(parserName string) Parser {
	switch parserName {
	case "rfc5424", "rfc3164", "json", "auto":
		return model.GetParser(parserName)
	default:
		return e.jsenv.GetParser(parserName)
	}
}

func (s *StreamServer) Register(r *consul.Registry) {
	if r == nil {
		return
	}
	for _, lc := range s.tcpListeners {
		svc := consul.NewService(lc.Conf.BindAddr, lc.Conf.Port, fmt.Sprintf("%s:%d", lc.Conf.BindAddr, lc.Conf.Port), []string{lc.Conf.Protocol})
		action := consul.ServiceAction{Action: consul.REGISTER, Service: svc}
		r.RegisterChan <- action
	}
}

func (s *StreamServer) Unregister(r *consul.Registry) {
	if r == nil {
		return
	}
	for _, lc := range s.tcpListeners {
		svc := consul.NewService(lc.Conf.BindAddr, lc.Conf.Port, fmt.Sprintf("%s:%d", lc.Conf.BindAddr, lc.Conf.Port), []string{lc.Conf.Protocol})
		action := consul.ServiceAction{Action: consul.UNREGISTER, Service: svc}
		r.RegisterChan <- action
	}
}

func (s *StreamServer) initTCPListeners() int {
	nb := 0
	s.connections = map[Connection]bool{}
	s.tcpListeners = []*TCPListenerConf{}
	s.unixListeners = []*UnixListenerConf{}
	for _, syslogConf := range s.Conf.Syslog {
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
				lc := UnixListenerConf{
					Listener: l,
					Conf:     syslogConf,
				}
				s.unixListeners = append(s.unixListeners, &lc)
			} else {
				s.logger.Error("Error listening on stream unix socket", "path", syslogConf.UnixSocketPath, "error", err)
				continue
			}
		} else {
			listenAddr, _ := syslogConf.GetListenAddr()
			tcpAddr, err := net.ResolveTCPAddr("tcp", listenAddr)
			if err != nil {
				s.logger.Warn("Error resolving TCP address", "address", listenAddr, "error", err)
				continue
			}
			l, err := net.ListenTCP("tcp", tcpAddr)
			if err == nil {
				s.logger.Info("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
				nb++
				lc := TCPListenerConf{
					Listener: l,
					Conf:     syslogConf,
				}
				s.tcpListeners = append(s.tcpListeners, &lc)
			} else {
				s.logger.Error("Error listening on stream (TCP or RELP)", "listen_addr", listenAddr, "error", err)
				continue
			}
		}
	}
	return nb
}

func (s *StreamServer) resetTCPListeners() {
	for _, l := range s.tcpListeners {
		l.Listener.Close()
	}
	for _, l := range s.unixListeners {
		l.Listener.Close()
	}
	s.tcpListeners = []*TCPListenerConf{}
	s.unixListeners = []*UnixListenerConf{}
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

func (s *StreamServer) handleConnection(conn net.Conn, config conf.SyslogConfig) {
	s.handler.HandleConnection(conn, config)
}

func (s *StreamServer) AcceptUnix(lc *UnixListenerConf) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, accept_err := lc.Listener.AcceptUnix()
		if accept_err != nil {
			s.logger.Info("AcceptUnix() error", "error", accept_err)
			switch accept_err.(type) {
			case *net.OpError:
				return
			default:
				// continue
			}
		} else if conn != nil {
			s.wg.Add(1)
			go s.handleConnection(conn, lc.Conf)
		}
	}

}

func (s *StreamServer) AcceptTCP(lc *TCPListenerConf) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, accept_err := lc.Listener.AcceptTCP()
		if accept_err != nil {
			s.logger.Info("AcceptTCP() error", "error", accept_err)
			switch accept_err.(type) {
			case *net.OpError:
				return
			default:
				// continue
			}
		} else if conn != nil {
			if lc.Conf.KeepAlive {
				err := conn.SetKeepAlive(true)
				if err == nil {
					err := conn.SetKeepAlivePeriod(lc.Conf.KeepAlivePeriod)
					if err != nil {
						s.logger.Warn("Error setting keepalive period", "addr", lc.Conf.BindAddr, "period", lc.Conf.KeepAlivePeriod)
					}
				} else {
					s.logger.Warn("Error setting keepalive", "addr", lc.Conf.BindAddr)
				}

			} else {
				err := conn.SetKeepAlive(false)
				if err != nil {
					s.logger.Warn("Error disabling keepalive", "addr", lc.Conf.BindAddr)
				}
			}
			err := conn.SetNoDelay(true)
			if err != nil {
				s.logger.Warn("Error setting TCP NODELAY", "addr", lc.Conf.BindAddr)
			}
			err = conn.SetLinger(-1)
			if err != nil {
				s.logger.Warn("Error setting TCP LINGER", "addr", lc.Conf.BindAddr)
			}
			s.wg.Add(1)
			go s.handleConnection(conn, lc.Conf)
		}
	}
}

func (s *StreamServer) Listen() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for _, lc := range s.tcpListeners {
			s.acceptsWg.Add(1)
			s.wg.Add(1)
			go s.AcceptTCP(lc)
		}
		for _, lc := range s.unixListeners {
			s.acceptsWg.Add(1)
			s.wg.Add(1)
			go s.AcceptUnix(lc)
		}
		// wait until the listeners stop and return
		s.acceptsWg.Wait()
		// close the client connections
		s.CloseConnections()
	}()
}
