package network

import (
	"crypto/tls"
	"net"
	"sync"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
)

type StreamHandler interface {
	HandleConnection(conn net.Conn, config conf.TcpSourceConfig)
}

type TCPListenerConf struct {
	Listener net.Listener
	Port     int
	Conf     conf.TcpSourceConfig
}

type UnixListenerConf struct {
	Listener net.Listener
	Conf     conf.TcpSourceConfig
}

type StreamingService struct {
	base.BaseService
	TcpConfigs     []conf.TcpSourceConfig
	TcpListeners   []TCPListenerConf
	UnixListeners  []UnixListenerConf
	handler        StreamHandler
	MaxMessageSize int
	acceptsWg      sync.WaitGroup
	wg             sync.WaitGroup
}

func (s *StreamingService) init() {
	s.BaseService.Init()
	s.TcpListeners = []TCPListenerConf{}
	s.UnixListeners = []UnixListenerConf{}
	s.TcpConfigs = []conf.TcpSourceConfig{}
}

func (s *StreamingService) initTCPListeners() []model.ListenerInfo {
	nb := 0
	s.ClearConnections()
	s.TcpListeners = []TCPListenerConf{}
	s.UnixListeners = []UnixListenerConf{}
	for _, syslogConf := range s.TcpConfigs {
		if len(syslogConf.UnixSocketPath) > 0 {
			l, err := s.Binder.Listen("unix", syslogConf.UnixSocketPath)
			if err != nil {
				s.Logger.Warn("Error listening on stream unix socket", "path", syslogConf.UnixSocketPath, "error", err)
			} else {
				s.Logger.Debug("Listener", "protocol", "stream", "path", syslogConf.UnixSocketPath, "format", syslogConf.Format)
				nb++
				lc := UnixListenerConf{
					Listener: l,
					Conf:     syslogConf,
				}
				s.UnixListeners = append(s.UnixListeners, lc)
				s.UnixSocketPaths = append(s.UnixSocketPaths, syslogConf.UnixSocketPath)
			}
		} else {
			listenAddrs, _ := syslogConf.GetListenAddrs()
			for port, listenAddr := range listenAddrs {
				l, err := s.Binder.Listen("tcp", listenAddr)
				if err != nil {
					s.Logger.Warn("Error listening on stream (TCP or RELP)", "listen_addr", listenAddr, "error", err)
				} else {
					s.Logger.Debug("Listener", "protocol", "stream", "addr", listenAddr, "format", syslogConf.Format)
					nb++
					lc := TCPListenerConf{
						Listener: l,
						Port:     port,
						Conf:     syslogConf,
					}
					s.TcpListeners = append(s.TcpListeners, lc)
				}
			}
		}
	}

	infos := []model.ListenerInfo{}
	for _, unixc := range s.UnixListeners {
		infos = append(infos, model.ListenerInfo{
			Protocol:       "tcp_or_relp",
			UnixSocketPath: unixc.Conf.UnixSocketPath,
		})
	}
	for _, tcpc := range s.TcpListeners {
		infos = append(infos, model.ListenerInfo{
			BindAddr: tcpc.Conf.BindAddr,
			Port:     tcpc.Port,
			Protocol: "tcp_or_relp",
		})
	}
	return infos
}

func (s *StreamingService) resetTCPListeners() {
	for _, l := range s.TcpListeners {
		_ = l.Listener.Close()
	}
	for _, l := range s.UnixListeners {
		_ = l.Listener.Close()
	}
}

func (s *StreamingService) handleConnection(conn net.Conn, config conf.TcpSourceConfig) {
	s.handler.HandleConnection(conn, config)
}

func (s *StreamingService) AcceptUnix(lc UnixListenerConf) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, accept_err := lc.Listener.Accept()
		if accept_err != nil {
			switch accept_err.(type) {
			case *net.OpError:
				s.Logger.Info("AcceptUnix() OpError", "error", accept_err)
			default:
				s.Logger.Warn("AcceptUnix() error", "error", accept_err)
			}
			return
		} else if conn != nil {
			s.wg.Add(1)
			go s.handleConnection(conn, lc.Conf)
		}
	}

}

func (s *StreamingService) AcceptTCP(lc TCPListenerConf) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		c, accept_err := lc.Listener.Accept()
		if accept_err != nil {
			switch accept_err.(type) {
			case *net.OpError:
				s.Logger.Info("AcceptTCP() OpError", "error", accept_err)
			default:
				s.Logger.Warn("AcceptTCP() error", "error", accept_err)
			}
			return
		} else if c != nil {
			if conn, ok := c.(*net.TCPConn); ok {
				if lc.Conf.KeepAlive {
					err := conn.SetKeepAlive(true)
					if err == nil {
						err := conn.SetKeepAlivePeriod(lc.Conf.KeepAlivePeriod)
						if err != nil {
							s.Logger.Warn("Error setting keepalive period", "addr", lc.Conf.BindAddr, "period", lc.Conf.KeepAlivePeriod)
						}
					} else {
						s.Logger.Warn("Error setting keepalive", "addr", lc.Conf.BindAddr)
					}

				} else {
					err := conn.SetKeepAlive(false)
					if err != nil {
						s.Logger.Warn("Error disabling keepalive", "addr", lc.Conf.BindAddr)
					}
				}
				err := conn.SetNoDelay(true)
				if err != nil {
					s.Logger.Warn("Error setting TCP NODELAY", "addr", lc.Conf.BindAddr)
				}
				err = conn.SetLinger(-1)
				if err != nil {
					s.Logger.Warn("Error setting TCP LINGER", "addr", lc.Conf.BindAddr)
				}
			}
			if lc.Conf.TLSEnabled {
				tlsConf, err := utils.NewTLSConfig("", lc.Conf.CAFile, lc.Conf.CAPath, lc.Conf.CertFile, lc.Conf.KeyFile, false)
				if err != nil {
					s.Logger.Warn("Error creating TLS configuration", "error", err)
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
		for _, lc := range s.TcpListeners {
			s.acceptsWg.Add(1)
			s.wg.Add(1)
			go s.AcceptTCP(lc)
		}
		for _, lc := range s.UnixListeners {
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

func (s *StreamingService) SetConf(sc []conf.TcpSourceConfig, pc []conf.ParserConfig, queueSize uint64, messageSize int) {
	s.MaxMessageSize = messageSize
	s.BaseService.SetConf(pc, queueSize)
	s.TcpConfigs = sc
}
