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
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
)

type UdpServerStatus int

const (
	UdpStopped UdpServerStatus = iota
	UdpStarted
)

type UdpServiceImpl struct {
	base.BaseService
	status     UdpServerStatus
	statusChan chan UdpServerStatus
	stasher    *base.Reporter
	handler    PacketHandler
	generator  chan ulid.ULID
	metrics    *udpMetrics
	registry   *prometheus.Registry
	wg         *sync.WaitGroup
}

type PacketHandler interface {
	HandleConnection(conn net.PacketConn, config conf.SyslogConfig)
}

type UdpHandler struct {
	Server *UdpServiceImpl
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
			Help: "total number of messages that were received",
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

func NewUdpService(stasher *base.Reporter, gen chan ulid.ULID, b *binder.BinderClient, l log15.Logger) *UdpServiceImpl {
	s := UdpServiceImpl{
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

func (s *UdpServiceImpl) SetConf(sc []conf.SyslogConfig, pc []conf.ParserConfig) {
	s.BaseService.Pool = &sync.Pool{New: func() interface{} {
		return &model.RawUdpMessage{}
	}}
	s.BaseService.SetConf(sc, pc)
}

func (s *UdpServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *UdpServiceImpl) handleConnection(conn net.PacketConn, config conf.SyslogConfig) {
	s.handler.HandleConnection(conn, config)
}

func (s *UdpServiceImpl) Start(test bool) ([]model.ListenerInfo, error) {
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

func (s *UdpServiceImpl) Shutdown() {
	s.Stop()
}

func (s *UdpServiceImpl) Stop() {
	s.LockStatus()
	if s.status != UdpStarted {
		s.UnlockStatus()
		return
	}
	s.CloseConnections()
	s.wg.Wait()

	s.status = UdpStopped
	s.statusChan <- UdpStopped
	close(s.statusChan)
	s.Logger.Debug("Udp server has stopped")
	s.UnlockStatus()
}

func (s *UdpServiceImpl) ListenPacket() []model.ListenerInfo {
	udpinfos := []model.ListenerInfo{}
	s.UnixSocketPaths = []string{}
	for _, syslogConf := range s.SyslogConfigs {
		if syslogConf.Protocol == "udp" {
			if len(syslogConf.UnixSocketPath) > 0 {
				conn, err := s.Binder.ListenPacket("unixgram", syslogConf.UnixSocketPath)
				if err != nil {
					s.Logger.Warn("Listen unixgram error", "error", err)
				} else {
					s.Logger.Debug(
						"Unixgram listener",
						"protocol", s.Protocol,
						"path", syslogConf.UnixSocketPath,
						"format", syslogConf.Format,
					)
					udpinfos = append(udpinfos, model.ListenerInfo{
						UnixSocketPath: syslogConf.UnixSocketPath,
						Protocol:       s.Protocol,
					})
					s.UnixSocketPaths = append(s.UnixSocketPaths, syslogConf.UnixSocketPath)
					conn.(*binder.FilePacketConn).PacketConn.(*net.UnixConn).SetReadBuffer(65536)
					conn.(*binder.FilePacketConn).PacketConn.(*net.UnixConn).SetWriteBuffer(65536)
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf)
				}
			} else {
				listenAddr, _ := syslogConf.GetListenAddr()
				conn, err := s.Binder.ListenPacket("udp", listenAddr)
				if err != nil {
					s.Logger.Warn("Listen UDP error", "error", err)
				} else {
					s.Logger.Debug(
						"UDP listener",
						"protocol", s.Protocol,
						"bind_addr", syslogConf.BindAddr,
						"port", syslogConf.Port,
						"format", syslogConf.Format,
					)
					udpinfos = append(udpinfos, model.ListenerInfo{
						BindAddr: syslogConf.BindAddr,
						Port:     syslogConf.Port,
						Protocol: syslogConf.Protocol,
					})
					conn.(*binder.FilePacketConn).PacketConn.(*net.UDPConn).SetReadBuffer(65536)
					conn.(*binder.FilePacketConn).PacketConn.(*net.UDPConn).SetReadBuffer(65536)
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf)
				}
			}
		}
	}
	return udpinfos
}

func (h UdpHandler) HandleConnection(conn net.PacketConn, config conf.SyslogConfig) {
	var localPort int
	var localPortS string
	var path string
	var err error
	rawMessages := make(chan *model.RawUdpMessage)

	s := h.Server
	s.AddConnection(conn)

	defer func() {
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	local := conn.LocalAddr()
	if local != nil {
		l := local.String()
		s := strings.Split(l, ":")
		localPort, err = strconv.Atoi(s[len(s)-1])
		if err != nil {
			path = strings.TrimSpace(l)
		} else {
			localPortS = strconv.FormatInt(int64(localPort), 10)
		}
	}

	logger := s.Logger.New("protocol", s.Protocol, "local_port", localPortS, "unix_socket_path", path)

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		e := NewParsersEnv(s.ParserConfigs, s.Logger)
		parser := e.GetParser(config.Format)
		if parser == nil {
			logger.Crit("Unknown parser", "format", config.Format)
			return
		}

		var syslogMsg model.SyslogMessage
		var err error
		var raw *model.RawUdpMessage
		decoder := utils.SelectDecoder(config.Encoding)

		for raw = range rawMessages {
			syslogMsg, err = parser.Parse(raw.Message[:raw.Size], decoder, config.DontParseSD)
			if err == nil {
				if !syslogMsg.Empty() {
					s.stasher.Stash(model.TcpUdpParsedMessage{
						Parsed: model.ParsedMessage{
							Fields:         syslogMsg,
							Client:         raw.Client,
							LocalPort:      raw.LocalPort,
							UnixSocketPath: raw.UnixSocketPath,
						},
						Uid:    <-s.generator,
						ConfId: config.ConfID,
					})
				}
			} else {
				s.metrics.ParsingErrorCounter.WithLabelValues(s.Protocol, raw.Client, config.Format).Inc()
				logger.Info("Parsing error", "client", raw.Client, "message", raw.Message, "error", err)
			}
			s.Pool.Put(raw)
		}
	}()

	// Syslog UDP server
	var remote net.Addr
	var rawmsg *model.RawUdpMessage

	for {
		rawmsg = s.Pool.Get().(*model.RawUdpMessage)
		rawmsg.Size, remote, err = conn.ReadFrom(rawmsg.Message[:])
		if err != nil {
			// todo: select more precise error
			logger.Debug("Error reading UDP", "error", err)
			s.Pool.Put(rawmsg)
			close(rawMessages)
			return
		}
		if rawmsg.Size == 0 {
			s.Pool.Put(rawmsg)
			continue
		}
		rawmsg.LocalPort = localPort
		rawmsg.UnixSocketPath = path
		rawmsg.Client = ""
		if remote == nil {
			// unix socket
			rawmsg.Client = "localhost"
		} else {
			rawmsg.Client = strings.Split(remote.String(), ":")[0]
		}

		// todo: use a bounded queue rather than a channel
		rawMessages <- rawmsg
		s.metrics.IncomingMsgsCounter.WithLabelValues(s.Protocol, rawmsg.Client, localPortS, path).Inc()
	}

}
