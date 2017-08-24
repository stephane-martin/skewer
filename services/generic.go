package services

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

type NetworkService interface {
	Start(test bool) ([]*model.ListenerInfo, error)
	Stop()
	WaitClosed()
	SetConf(sc []*conf.SyslogConfig, pc []conf.ParserConfig)
	SetKafkaConf(kc *conf.KafkaConfig)
	SetAuditConf(ac *conf.AuditConfig)
	Gather() ([]*dto.MetricFamily, error)
}

func NewNetworkService(t string, stasher model.Stasher, gen chan ulid.ULID, b *sys.BinderClient, l log15.Logger) (NetworkService, context.CancelFunc) {
	switch t {
	case "skewer-tcp":
		return NewTcpService(stasher, gen, b, l), nil
	case "skewer-udp":
		return NewUdpService(stasher, gen, b, l), nil
	case "skewer-relp":
		return NewRelpService(b, l), nil
	case "skewer-journal":
		ctx, cancel := context.WithCancel(context.Background())
		s, err := NewJournalService(ctx, stasher, gen, l)
		if err == nil {
			return s, cancel
		} else {
			l.Error("Error initializing journal service", "error", err)
			cancel()
			return nil, nil
		}
	case "skewer-audit":
		return NewAuditService(stasher, gen, l), nil
	default:
		return nil, nil
	}
}

type Parser interface {
	Parse(m string, dont_parse_sd bool) (*model.SyslogMessage, error)
}

type StreamHandler interface {
	HandleConnection(conn net.Conn, config *conf.SyslogConfig)
}

type Connection interface {
	Close() error
}

type GenericService struct {
	SyslogConfigs   []*conf.SyslogConfig
	ParserConfigs   []conf.ParserConfig
	logger          log15.Logger
	binder          *sys.BinderClient
	unixSocketPaths []string
	wg              *sync.WaitGroup
	protocol        string
	connections     map[Connection]bool
	connMutex       *sync.Mutex
	statusMutex     *sync.Mutex
}

func (s *GenericService) init() {
	s.wg = &sync.WaitGroup{}
	s.unixSocketPaths = []string{}
	s.connMutex = &sync.Mutex{}
	s.connections = map[Connection]bool{}
	s.statusMutex = &sync.Mutex{}
}

func (s *GenericService) SetConf(sc []*conf.SyslogConfig, pc []conf.ParserConfig) {
	s.SyslogConfigs = sc
	s.ParserConfigs = pc
}

type TCPListenerConf struct {
	Listener net.Listener
	Conf     *conf.SyslogConfig
}

type UnixListenerConf struct {
	Listener net.Listener
	Conf     *conf.SyslogConfig
}

type StreamingService struct {
	GenericService
	tcpListeners  []*TCPListenerConf
	unixListeners []*UnixListenerConf
	acceptsWg     *sync.WaitGroup
	handler       StreamHandler
}

func (s *StreamingService) init() {
	s.GenericService.init()
	s.tcpListeners = []*TCPListenerConf{}
	s.unixListeners = []*UnixListenerConf{}
	s.acceptsWg = &sync.WaitGroup{}
}

type ParsersEnv struct {
	jsenv javascript.ParsersEnvironment
}

func NewParsersEnv(parsersConf []conf.ParserConfig, logger log15.Logger) *ParsersEnv {
	p := javascript.NewParsersEnvironment(logger)
	for _, parserConf := range parsersConf {
		err := p.AddParser(parserConf.Name, parserConf.Func)
		if err != nil {
			logger.Warn("Error initializing parser", "name", parserConf.Name, "error", err)
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

func (s *StreamingService) initTCPListeners() []*model.ListenerInfo {
	nb := 0
	s.connections = map[Connection]bool{}
	s.tcpListeners = []*TCPListenerConf{}
	s.unixListeners = []*UnixListenerConf{}
	//fmt.Println(s.SyslogConfigs)
	for _, syslogConf := range s.SyslogConfigs {
		if syslogConf.Protocol != s.protocol {
			continue
		}
		if len(syslogConf.UnixSocketPath) > 0 {
			l, err := net.Listen("unix", syslogConf.UnixSocketPath)
			if err != nil {
				if s.binder == nil {
					s.logger.Warn("Error listening on stream unix socket", "path", syslogConf.UnixSocketPath, "error", err)
					l = nil
				} else {
					s.logger.Info("Error listening on stream unix socket. Retrying as root.", "path", syslogConf.UnixSocketPath, "error", err)
					l, err = s.binder.Listen("unix", syslogConf.UnixSocketPath)
					if err != nil {
						s.logger.Warn("Parent could not listen either", "path", syslogConf.UnixSocketPath, "error", err)
						l = nil
					}
				}
			}
			if l != nil {
				s.logger.Debug("Listener", "protocol", s.protocol, "path", syslogConf.UnixSocketPath, "format", syslogConf.Format)
				nb++
				lc := UnixListenerConf{
					Listener: l,
					Conf:     syslogConf,
				}
				s.unixListeners = append(s.unixListeners, &lc)
				s.unixSocketPaths = append(s.unixSocketPaths, syslogConf.UnixSocketPath)
			}
		} else {
			listenAddr, _ := syslogConf.GetListenAddr()
			l, err := net.Listen("tcp", listenAddr)
			if err != nil {
				if s.binder == nil || syslogConf.Port > 1024 {
					s.logger.Warn("Error listening on stream (TCP or RELP)", "listen_addr", listenAddr, "error", err)
					l = nil
				} else {
					s.logger.Info("Error listening on stream (TCP or RELP). Retrying as root.", "listen_addr", listenAddr, "error", err)
					l, err = s.binder.Listen("tcp", listenAddr)
					if err != nil {
						s.logger.Warn("Parent could not listen either", "listen_addr", listenAddr, "error", err)
						l = nil
					}
				}
			}
			if l != nil {
				s.logger.Debug("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
				nb++
				lc := TCPListenerConf{
					Listener: l,
					Conf:     syslogConf,
				}
				s.tcpListeners = append(s.tcpListeners, &lc)
			}
		}
	}

	infos := []*model.ListenerInfo{}
	for _, unixc := range s.unixListeners {
		infos = append(infos, &model.ListenerInfo{
			Protocol:       unixc.Conf.Protocol,
			UnixSocketPath: unixc.Conf.UnixSocketPath,
		})
	}
	for _, tcpc := range s.tcpListeners {
		infos = append(infos, &model.ListenerInfo{
			BindAddr: tcpc.Conf.BindAddr,
			Port:     tcpc.Conf.Port,
			Protocol: tcpc.Conf.Protocol,
		})
	}

	return infos
}

func (s *StreamingService) resetTCPListeners() {
	for _, l := range s.tcpListeners {
		l.Listener.Close()
	}
	for _, l := range s.unixListeners {
		l.Listener.Close()
	}
}

func (s *GenericService) AddConnection(conn Connection) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	s.connections[conn] = true
}

func (s *GenericService) RemoveConnection(conn Connection) {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	if _, ok := s.connections[conn]; ok {
		conn.Close()
		delete(s.connections, conn)
	}
}

func (s *GenericService) CloseConnections() {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()
	for conn, _ := range s.connections {
		conn.Close()
	}
	for _, path := range s.unixSocketPaths {
		if !strings.HasPrefix(path, "@") {
			os.Remove(path)
		}
	}
	s.connections = map[Connection]bool{}
	s.unixSocketPaths = []string{}
}

func (s *StreamingService) handleConnection(conn net.Conn, config *conf.SyslogConfig) {
	s.handler.HandleConnection(conn, config)
}

func (s *StreamingService) AcceptUnix(lc *UnixListenerConf) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, accept_err := lc.Listener.Accept()
		if accept_err != nil {
			switch accept_err.(type) {
			case *net.OpError:
				s.logger.Info("AcceptUnix() OpError", "error", accept_err)
			default:
				s.logger.Warn("AcceptUnix() error", "error", accept_err)
			}
			return
		} else if conn != nil {
			s.wg.Add(1)
			go s.handleConnection(conn, lc.Conf)
		}
	}

}

func (s *StreamingService) AcceptTCP(lc *TCPListenerConf) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		c, accept_err := lc.Listener.Accept()
		if accept_err != nil {
			switch accept_err.(type) {
			case *net.OpError:
				s.logger.Info("AcceptTCP() OpError", "error", accept_err)
			default:
				s.logger.Warn("AcceptTCP() error", "error", accept_err)
			}
			return
		} else if c != nil {
			if conn, ok := c.(*net.TCPConn); ok {
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
			}
			if lc.Conf.TLSEnabled {
				tlsConf, err := utils.NewTLSConfig("", lc.Conf.CAFile, lc.Conf.CAPath, lc.Conf.CertFile, lc.Conf.KeyFile, false)
				if err != nil {
					s.logger.Warn("Error creating TLS configuration", "error", err)
				} else {
					tlsConf.ClientAuth = lc.Conf.GetClientAuthType()
					s.wg.Add(1)
					go s.handleConnection(tls.Server(c, tlsConf), lc.Conf)
				}

			} else {
				s.wg.Add(1)
				go s.handleConnection(c, lc.Conf)
			}
		}
	}
}

func (s *StreamingService) Listen() {
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
