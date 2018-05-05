package network

import (
	"crypto/tls"
	"net"
	"sync"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

type StreamHandler interface {
	HandleConnection(conn net.Conn, config conf.TCPSourceConfig) error
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

func (s *StreamingService) handleConnection(conn net.Conn, config conf.TCPSourceConfig) error {
	return s.handler.HandleConnection(conn, config)
}

func (s *StreamingService) AcceptUnix(lc UnixListenerConf) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		conn, err := lc.Listener.Accept()
		if err != nil {
			return eerrors.Wrap(err, "Accept() error")
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.handleConnection(conn, lc.Conf)
			if err != nil && !eerrors.HasFileClosed(err) {
				s.Logger.Warn("Unix connection error", "error", err)
			}
		}()
	}

}

func (s *StreamingService) AcceptTCP(lc TCPListenerConf) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		c, err := lc.Listener.Accept()
		if err != nil {
			return eerrors.Wrap(err, "Accept() error")
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
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.handleConnection(c, lc.Conf)
			if err != nil && !eerrors.HasFileClosed(err) {
				s.Logger.Warn("TCP connection error", "error", err)
			}
		}()
	}
}

func (s *StreamingService) Listen() (err error) {
	c := eerrors.ChainErrors()
	var wg sync.WaitGroup

	for _, lc := range s.TcpListeners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Append(s.AcceptTCP(lc))
		}()
	}
	for _, lc := range s.UnixListeners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Append(s.AcceptUnix(lc))
		}()
	}
	wg.Wait()
	errs := c.Sum()
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (s *StreamingService) SetConf(sc []conf.TCPSourceConfig, pc []conf.ParserConfig, queueSize uint64, messageSize int) {
	s.MaxMessageSize = messageSize
	s.BaseService.SetConf(pc, queueSize)
	s.TcpConfigs = sc
}
