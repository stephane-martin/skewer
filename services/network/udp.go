package network

import (
	"net"
	"runtime"
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
	"github.com/stephane-martin/skewer/utils/queue/udp"
	"golang.org/x/text/encoding"
)

type UdpServerStatus int

const (
	UdpStopped UdpServerStatus = iota
	UdpStarted
)

type UdpServiceImpl struct {
	base.BaseService
	UdpConfigs []conf.UdpSourceConfig
	status     UdpServerStatus
	statusChan chan UdpServerStatus
	stasher    *base.Reporter
	handler    PacketHandler
	generator  chan ulid.ULID
	metrics    *udpMetrics
	registry   *prometheus.Registry
	wg         sync.WaitGroup

	rawMessagesQueue *udp.Ring
}

type PacketHandler interface {
	HandleConnection(conn net.PacketConn, config conf.UdpSourceConfig)
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
		status:     UdpStopped,
		metrics:    NewUdpMetrics(),
		registry:   prometheus.NewRegistry(),
		stasher:    stasher,
		generator:  gen,
		UdpConfigs: []conf.UdpSourceConfig{},
	}
	s.BaseService.Init()
	s.registry.MustRegister(s.metrics.IncomingMsgsCounter, s.metrics.ParsingErrorCounter)
	s.BaseService.Logger = l.New("class", "UdpServer")
	s.BaseService.Binder = b
	s.handler = UdpHandler{Server: &s}
	return &s
}

func (s *UdpServiceImpl) SetConf(sc []conf.UdpSourceConfig, pc []conf.ParserConfig, queueSize uint64) {
	s.BaseService.Pool = &sync.Pool{New: func() interface{} {
		return &model.RawUdpMessage{}
	}}
	s.BaseService.SetConf(pc, queueSize)
	s.UdpConfigs = sc
	s.rawMessagesQueue = udp.NewRing(s.QueueSize)
}

// Parse fetch messages from the raw queue, parse them, and push them to be sent.
func (s *UdpServiceImpl) Parse() {
	defer s.wg.Done()

	e := NewParsersEnv(s.ParserConfigs, s.Logger)

	var syslogMsg *model.SyslogMessage
	var err, fatal, nonfatal error
	var raw *model.RawUdpMessage
	var decoder *encoding.Decoder
	var parser Parser
	var logger log15.Logger

	for {
		raw, err = s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			break
		}

		logger = s.Logger.New(
			"protocol", "udp",
			"client", raw.Client,
			"local_port", raw.LocalPort,
			"unix_socket_path", raw.UnixSocketPath,
			"format", raw.Format,
		)

		decoder = utils.SelectDecoder(raw.Encoding)
		parser = e.GetParser(raw.Format)
		if parser == nil {
			logger.Crit("Unknown parser")
			return
		}

		syslogMsg, err = parser.Parse(raw.Message[:raw.Size], decoder, raw.DontParseSD)
		if err != nil {
			s.Pool.Put(raw)
			s.metrics.ParsingErrorCounter.WithLabelValues("udp", raw.Client, raw.Format).Inc()
			logger.Info("Parsing error", "Message", raw.Message, "error", err)
			continue
		}
		if syslogMsg == nil {
			s.Pool.Put(raw)
			continue
		}

		fatal, nonfatal = s.stasher.Stash(model.FullMessage{
			Parsed: model.ParsedMessage{
				Fields:         *syslogMsg,
				Client:         raw.Client,
				LocalPort:      raw.LocalPort,
				UnixSocketPath: raw.UnixSocketPath,
			},
			Uid:    <-s.generator,
			ConfId: raw.ConfID,
		})

		s.Pool.Put(raw)

		if fatal != nil {
			logger.Error("Fatal error stashing UDP message", "error", fatal)
			// todo: shutdown
		} else if nonfatal != nil {
			logger.Warn("Non-fatal error stashing UDP message", "error", nonfatal)
		}
	}
}

func (s *UdpServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *UdpServiceImpl) handleConnection(conn net.PacketConn, config conf.UdpSourceConfig) {
	s.handler.HandleConnection(conn, config)
}

func (s *UdpServiceImpl) Start(test bool) ([]model.ListenerInfo, error) {
	s.LockStatus()
	if s.status != UdpStopped {
		s.UnlockStatus()
		return nil, errors.ServerNotStopped
	}
	s.statusChan = make(chan UdpServerStatus, 1)

	// start the parsers
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.wg.Add(1)
		go s.Parse()
	}

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
	if s.rawMessagesQueue != nil {
		s.rawMessagesQueue.Dispose()
	}
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
	for _, syslogConf := range s.UdpConfigs {
		if len(syslogConf.UnixSocketPath) > 0 {
			conn, err := s.Binder.ListenPacket("unixgram", syslogConf.UnixSocketPath)
			if err != nil {
				s.Logger.Warn("Listen unixgram error", "error", err)
			} else {
				s.Logger.Debug(
					"Unixgram listener",
					"protocol", "udp",
					"path", syslogConf.UnixSocketPath,
					"format", syslogConf.Format,
				)
				udpinfos = append(udpinfos, model.ListenerInfo{
					UnixSocketPath: syslogConf.UnixSocketPath,
					Protocol:       "udp",
				})
				s.UnixSocketPaths = append(s.UnixSocketPaths, syslogConf.UnixSocketPath)
				conn.(*binder.FilePacketConn).PacketConn.(*net.UnixConn).SetReadBuffer(65536)
				conn.(*binder.FilePacketConn).PacketConn.(*net.UnixConn).SetWriteBuffer(65536)
				s.wg.Add(1)
				go s.handleConnection(conn, syslogConf)
			}
		} else {
			listenAddrs, _ := syslogConf.GetListenAddrs()
			for port, listenAddr := range listenAddrs {
				conn, err := s.Binder.ListenPacket("udp", listenAddr)
				if err != nil {
					s.Logger.Warn("Listen UDP error", "error", err)
				} else {
					s.Logger.Debug(
						"UDP listener",
						"protocol", "udp",
						"bind_addr", syslogConf.BindAddr,
						"port", port,
						"format", syslogConf.Format,
					)
					udpinfos = append(udpinfos, model.ListenerInfo{
						BindAddr: syslogConf.BindAddr,
						Port:     port,
						Protocol: "udp",
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

func (h UdpHandler) HandleConnection(conn net.PacketConn, config conf.UdpSourceConfig) {
	var localPort int
	var localPortS string
	var path string
	var err error
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

	logger := s.Logger.New("protocol", "udp", "local_port", localPortS, "unix_socket_path", path)

	// Syslog UDP server
	var remote net.Addr
	var rawmsg *model.RawUdpMessage

	for {
		rawmsg = s.Pool.Get().(*model.RawUdpMessage)
		rawmsg.Size, remote, err = conn.ReadFrom(rawmsg.Message[:])
		if err != nil {
			logger.Debug("Error reading UDP", "error", err)
			s.Pool.Put(rawmsg)
			break
		}
		if rawmsg.Size == 0 {
			s.Pool.Put(rawmsg)
			continue
		}
		rawmsg.LocalPort = localPort
		rawmsg.UnixSocketPath = path
		rawmsg.Encoding = config.Encoding
		rawmsg.Format = config.Format
		rawmsg.ConfID = config.ConfID
		rawmsg.DontParseSD = config.DontParseSD
		rawmsg.Client = ""
		if remote == nil {
			// unix socket
			rawmsg.Client = "localhost"
		} else {
			rawmsg.Client = strings.Split(remote.String(), ":")[0]
		}

		s.rawMessagesQueue.Put(rawmsg)
		s.metrics.IncomingMsgsCounter.WithLabelValues("udp", rawmsg.Client, localPortS, path).Inc()
	}
}
