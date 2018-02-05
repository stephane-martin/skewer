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
	HandleConnection(conn net.Conn, config conf.TCPSourceConfig)
}

type TCPListenerConf struct {
	Listener net.Listener
	Port     int
	Conf     conf.TCPSourceConfig
}

type UnixListenerConf struct {
	Listener net.Listener
	Conf     conf.TCPSourceConfig
}

type StreamingService struct {
	base.BaseService
	TcpConfigs     []conf.TCPSourceConfig
	TcpListeners   []TCPListenerConf
	UnixListeners  []UnixListenerConf
	handler        StreamHandler
	MaxMessageSize int
	acceptsWg      sync.WaitGroup
	wg             sync.WaitGroup
	confined       bool
}

func (s *StreamingService) init() {
	s.BaseService.Init()
	s.TcpListeners = []TCPListenerConf{}
	s.UnixListeners = []UnixListenerConf{}
	s.TcpConfigs = []conf.TCPSourceConfig{}
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
				var l net.Listener
				var err error
				if syslogConf.KeepAlive {
					l, err = s.Binder.ListenKeepAlive("tcp", listenAddr, syslogConf.KeepAlivePeriod)
				} else {
					l, err = s.Binder.Listen("tcp", listenAddr)
				}
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

func (s *StreamingService) handleConnection(conn net.Conn, config conf.TCPSourceConfig) {
	s.handler.HandleConnection(conn, config)
}

func (s *StreamingService) AcceptUnix(lc UnixListenerConf) {
	defer s.wg.Done()
	defer s.acceptsWg.Done()
	for {
		conn, err := lc.Listener.Accept()
		if err != nil {
			switch err.(type) {
			case *net.OpError:
				s.Logger.Info("AcceptUnix() OpError", "error", err)
			default:
				s.Logger.Warn("AcceptUnix() error", "error", err)
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
		c, err := lc.Listener.Accept()
		if err != nil {
			switch err.(type) {
			case *net.OpError:
				s.Logger.Info("AcceptTCP() OpError", "error", err)
			default:
				s.Logger.Warn("AcceptTCP() error", "error", err)
			}
			return
		}
		if lc.Conf.TLSEnabled {
			// upgrade connection to TLS
			tlsConf, err := utils.NewTLSConfig("", lc.Conf.CAFile, lc.Conf.CAPath, lc.Conf.CertFile, lc.Conf.KeyFile, false, s.confined)
			if err != nil {
				s.Logger.Warn("Error creating TLS configuration", "error", err)
				continue
			}
			tlsConf.ClientAuth = lc.Conf.GetClientAuthType()
			c = tls.Server(c, tlsConf)
		}
		s.wg.Add(1)
		go s.handleConnection(c, lc.Conf)
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

func (s *StreamingService) SetConf(sc []conf.TCPSourceConfig, pc []conf.ParserConfig, queueSize uint64, messageSize int) {
	s.MaxMessageSize = messageSize
	s.BaseService.SetConf(pc, queueSize)
	s.TcpConfigs = sc
}
