package network

import (
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/services/errors"
	"github.com/stephane-martin/skewer/sys"
)

type UdpServerStatus int

const (
	UdpStopped UdpServerStatus = iota
	UdpStarted
)

type udpServiceImpl struct {
	base.BaseService
	status     UdpServerStatus
	statusChan chan UdpServerStatus
	stasher    model.Stasher
	handler    PacketHandler
	generator  chan ulid.ULID
	metrics    *udpMetrics
	registry   *prometheus.Registry
	wg         *sync.WaitGroup
}

type PacketHandler interface {
	HandleConnection(conn net.PacketConn, config *conf.SyslogConfig)
}

type UdpHandler struct {
	Server *udpServiceImpl
}

type udpMetrics struct {
	IncomingMsgsCounter *prometheus.CounterVec
	ParsingErrorCounter *prometheus.CounterVec
}

func NewUdpMetrics() *udpMetrics {
	m := &udpMetrics{}
	m.IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_incoming_messages_total",
			Help: "total number of syslog messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)
	m.ParsingErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_parsing_errors_total",
			Help: "total number of times there was a parsing error",
		},
		[]string{"protocol", "client", "parser_name"},
	)
	return m
}

func NewUdpService(stasher model.Stasher, gen chan ulid.ULID, b *sys.BinderClient, l log15.Logger) *udpServiceImpl {
	s := udpServiceImpl{
		status:    UdpStopped,
		metrics:   NewUdpMetrics(),
		registry:  prometheus.NewRegistry(),
		stasher:   stasher,
		generator: gen,
		wg:        &sync.WaitGroup{},
	}
	s.BaseService.Init()
	s.registry.MustRegister(s.metrics.IncomingMsgsCounter, s.metrics.ParsingErrorCounter)
	s.BaseService.Logger = l.New("class", "UdpServer")
	s.BaseService.Binder = b
	s.BaseService.Protocol = "udp"
	s.handler = UdpHandler{Server: &s}
	return &s
}

func (s *udpServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *udpServiceImpl) handleConnection(conn net.PacketConn, config *conf.SyslogConfig) {
	s.handler.HandleConnection(conn, config)
}

func (s *udpServiceImpl) SetKafkaConf(kc *conf.KafkaConfig) {}

func (s *udpServiceImpl) SetAuditConf(ac *conf.AuditConfig) {}

func (s *udpServiceImpl) Start(test bool) ([]*model.ListenerInfo, error) {
	s.LockStatus()
	if s.status != UdpStopped {
		s.UnlockStatus()
		return nil, errors.ServerNotStopped
	}
	s.statusChan = make(chan UdpServerStatus, 1)

	s.ClearConnections()
	infos := s.ListenPacket()
	if len(infos) > 0 {
		s.status = UdpStarted
		s.Logger.Info("Listening on UDP", "nb_services", len(infos))
	} else {
		s.Logger.Debug("The UDP service has not been started: no listening port")
		close(s.statusChan)
	}
	s.UnlockStatus()
	return infos, nil
}

func (s *udpServiceImpl) Stop() {
	s.LockStatus()
	if s.status != UdpStarted {
		s.UnlockStatus()
		return
	}
	s.Logger.Debug("Closing UDP connections")
	s.CloseConnections()
	s.Logger.Debug("Waiting for UDP goroutines")
	s.wg.Wait()
	s.Logger.Debug("UdpServer goroutines have ended")

	s.status = UdpStopped
	s.statusChan <- UdpStopped
	close(s.statusChan)
	s.Logger.Debug("Udp server has stopped")
	s.UnlockStatus()
}

func (s *udpServiceImpl) WaitClosed() {
	var more bool
	for {
		_, more = <-s.statusChan
		if !more {
			return
		}
	}
}

func (s *udpServiceImpl) ListenPacket() []*model.ListenerInfo {
	udpinfos := []*model.ListenerInfo{}
	s.UnixSocketPaths = []string{}
	for _, syslogConf := range s.SyslogConfigs {
		if syslogConf.Protocol == "udp" {
			if len(syslogConf.UnixSocketPath) > 0 {
				conn, err := net.ListenPacket("unixgram", syslogConf.UnixSocketPath)
				if err != nil {
					switch err.(type) {
					case *net.OpError:
						if s.Binder == nil {
							s.Logger.Warn("Listen unixgram OpError", "error", err)
							conn = nil
						} else {
							s.Logger.Info("Listen unixgram OpError. Retrying as root.", "error", err)
							conn, err = s.Binder.ListenPacket("unixgram", syslogConf.UnixSocketPath)
							if err != nil {
								s.Logger.Warn("Listen unixgram OpError", "error", err)
								conn = nil
							}
						}
					default:
						s.Logger.Warn("Listen unixgram error", "error", err)
						conn = nil
					}
				}
				if conn != nil && err == nil {
					s.Logger.Debug("Listener", "protocol", s.Protocol, "path", syslogConf.UnixSocketPath, "format", syslogConf.Format)
					udpinfos = append(udpinfos, &model.ListenerInfo{
						UnixSocketPath: syslogConf.UnixSocketPath,
						Protocol:       s.Protocol,
					})
					s.UnixSocketPaths = append(s.UnixSocketPaths, syslogConf.UnixSocketPath)
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf)
				}
			} else {
				listenAddr, _ := syslogConf.GetListenAddr()
				conn, err := net.ListenPacket("udp", listenAddr)
				if err != nil {
					switch err.(type) {
					case *net.OpError:
						if s.Binder == nil || syslogConf.Port > 1024 {
							s.Logger.Warn("Listen UDP OpError", "error", err)
							conn = nil
						} else {
							s.Logger.Info("Listen unixgram OpError. Retrying as root.", "error", err)
							conn, err = s.Binder.ListenPacket("udp", listenAddr)
							if err != nil {
								s.Logger.Warn("Listen UDP OpError", "error", err)
								conn = nil
							}
						}
					default:
						s.Logger.Warn("Listen UDP error", "error", err)
						conn = nil
					}

				}
				if conn != nil && err == nil {
					s.Logger.Debug("Listener", "protocol", s.Protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
					udpinfos = append(udpinfos, &model.ListenerInfo{
						BindAddr: syslogConf.BindAddr,
						Port:     syslogConf.Port,
						Protocol: syslogConf.Protocol,
					})
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf)
				}
			}
		}
	}
	return udpinfos
}

func (h UdpHandler) HandleConnection(conn net.PacketConn, config *conf.SyslogConfig) {
	var local_port int
	var err error

	s := h.Server
	s.AddConnection(conn)

	raw_messages_chan := make(chan *model.RawMessage)

	defer func() {
		close(raw_messages_chan)
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	path := ""
	local := conn.LocalAddr()
	if local != nil {
		l := local.String()
		s := strings.Split(l, ":")
		local_port, err = strconv.Atoi(s[len(s)-1])
		if err != nil {
			path = l
		}
	}
	path = strings.TrimSpace(path)
	local_port_s := strconv.FormatInt(int64(local_port), 10)

	logger := s.Logger.New("protocol", s.Protocol, "local_port", local_port, "unix_socket_path", path, "format", config.Format)

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		e := NewParsersEnv(s.ParserConfigs, s.Logger)
		for m := range raw_messages_chan {
			parser := e.GetParser(config.Format)
			if parser == nil {
				logger.Error("Unknown parser", "client", m.Client)
				continue
			}
			p, err := parser.Parse(m.Message, config.DontParseSD)

			if err == nil {
				uid := <-s.generator
				parsed_msg := model.TcpUdpParsedMessage{
					Parsed: &model.ParsedMessage{
						Fields:         p,
						Client:         m.Client,
						LocalPort:      m.LocalPort,
						UnixSocketPath: m.UnixSocketPath,
					},
					Uid:    uid.String(),
					ConfId: config.ConfID,
				}
				s.stasher.Stash(&parsed_msg)
			} else {
				if s.metrics != nil {
					s.metrics.ParsingErrorCounter.WithLabelValues(s.Protocol, m.Client, config.Format).Inc()
				}
				logger.Info("Parsing error", "client", m.Client, "message", m.Message, "error", err)
			}
		}
	}()

	// Syslog UDP server
	for {
		packet := make([]byte, 65536)
		size, remote, err := conn.ReadFrom(packet)
		if err != nil {
			logger.Debug("Error reading UDP", "error", err)
			return
		}
		client := ""
		if remote == nil {
			// unix socket
			client = "localhost"
		} else {
			client = strings.Split(remote.String(), ":")[0]
		}

		raw := model.RawMessage{
			Client:         client,
			LocalPort:      local_port,
			UnixSocketPath: path,
			Message:        string(packet[:size]),
		}
		if s.metrics != nil {
			s.metrics.IncomingMsgsCounter.WithLabelValues(s.Protocol, client, local_port_s, path).Inc()
		}
		raw_messages_chan <- &raw
	}

}
