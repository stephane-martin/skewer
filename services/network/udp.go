package network

import (
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"

	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue/udp"
)

func initUdpRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type UdpServiceImpl struct {
	base.BaseService
	UdpConfigs       []conf.UDPSourceConfig
	stasher          *base.Reporter
	wg               sync.WaitGroup
	fatalErrorChan   chan struct{}
	fatalOnce        *sync.Once
	parserEnv        *decoders.ParsersEnv
	rawMessagesQueue *udp.Ring
}

func NewUdpService(env *base.ProviderEnv) (*UdpServiceImpl, error) {
	initUdpRegistry()
	s := UdpServiceImpl{
		stasher:    env.Reporter,
		UdpConfigs: []conf.UDPSourceConfig{},
	}
	s.BaseService.Init()
	s.BaseService.Logger = env.Logger.New("class", "UdpServer")
	s.BaseService.Binder = env.Binder
	return &s, nil
}

func (s *UdpServiceImpl) Type() base.Types {
	return base.UDP
}

//func (s *UdpServiceImpl) SetConf(sc []conf.UDPSourceConfig, pc []conf.ParserConfig, queueSize uint64) {
func (s *UdpServiceImpl) SetConf(c conf.BaseConfig) {
	s.BaseService.SetConf(c.Parsers, c.Main.InputQueueSize)
	s.UdpConfigs = c.UDPSource
	s.rawMessagesQueue = udp.NewRing(c.Main.InputQueueSize)
	s.parserEnv = decoders.NewParsersEnv(s.ParserConfigs, s.Logger)
}

// Parse fetch messages from the raw queue, parse them, and push them to be sent.
func (s *UdpServiceImpl) Parse() error {
	gen := utils.NewGenerator()

	for {
		raw, err := s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			return nil
		}
		err = s.ParseOne(raw, gen)
		if err != nil {
			base.CountParsingError(base.UDP, raw.Client, raw.Decoder.Format)
			logg(s.Logger, &raw.RawMessage).Warn(err.Error())
		}
		model.RawUDPFree(raw)

		if err != nil && eerrors.IsFatal(err) {
			// stop processing when fatal error happens
			return err
		}
	}
}

func (s *UdpServiceImpl) ParseOne(raw *model.RawUDPMessage, gen *utils.Generator) error {
	syslogMsgs, err := s.parserEnv.Parse(&raw.Decoder, raw.Message[:raw.Size])
	if err != nil {
		return err
	}

	for _, syslogMsg := range syslogMsgs {
		if syslogMsg == nil {
			continue
		}
		full := model.FullFactoryFrom(syslogMsg)
		full.Uid = gen.Uid()
		full.ConfId = raw.ConfID
		full.SourceType = "udp"
		full.SourcePath = raw.UnixSocketPath
		full.SourcePort = int32(raw.LocalPort)
		full.ClientAddr = raw.Client
		err := s.stasher.Stash(full)
		model.FullFree(full)

		if err != nil {
			logg(s.Logger, &raw.RawMessage).Warn("Error stashing UDP message", "error", err)
			if eerrors.IsFatal(err) {
				return eerrors.Wrap(err, "Fatal error pushing UDP message to the Store")
			}
		}
	}
	return nil
}

func (s *UdpServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *UdpServiceImpl) Start() ([]model.ListenerInfo, error) {
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}

	// start the parsers
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := s.Parse()
			if err != nil {
				s.dofatal()
				s.Logger.Error(err.Error())
			}
		}()
	}

	s.ClearConnections()
	c := make(chan model.ListenerInfo)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.ListenPacket(c)
	}()
	infos := make([]model.ListenerInfo, 0)
	for i := range c {
		infos = append(infos, i)
	}
	if len(infos) > 0 {
		s.Logger.Info("Listening on UDP", "nb_services", len(infos))
	} else {
		s.Logger.Debug("The UDP service has not been started: no listening port")
	}
	return infos, nil
}

func (s *UdpServiceImpl) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *UdpServiceImpl) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *UdpServiceImpl) Shutdown() {
	s.Stop()
}

func (s *UdpServiceImpl) Stop() {
	s.CloseConnections()
	if s.rawMessagesQueue != nil {
		s.rawMessagesQueue.Dispose()
	}
	s.wg.Wait()
	s.Logger.Debug("Udp server has stopped")
}

func (s *UdpServiceImpl) ListenPacket(c chan model.ListenerInfo) {
	var wg sync.WaitGroup
	s.UnixSocketPaths = []string{}

	for _, syslogConf := range s.UdpConfigs {
		if len(syslogConf.UnixSocketPath) > 0 {
			conn, err := s.Binder.ListenPacket("unixgram", syslogConf.UnixSocketPath, 65536)
			if err != nil {
				s.Logger.Warn("Listen unixgram error", "error", err)
				continue
			}
			s.Logger.Debug(
				"Unixgram listener",
				"protocol", "udp",
				"path", syslogConf.UnixSocketPath,
				"format", syslogConf.Format,
			)
			c <- model.ListenerInfo{
				UnixSocketPath: syslogConf.UnixSocketPath,
				Protocol:       "udp",
			}
			s.UnixSocketPaths = append(s.UnixSocketPaths, syslogConf.UnixSocketPath)
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := s.handleConnection(conn, syslogConf)
				if err != nil && !eerrors.HasFileClosed(err) {
					s.Logger.Warn("Unix datagram connection error", "error", err)
				}
			}()
		} else {
			listenAddrs, err := syslogConf.GetListenAddrs()
			if err != nil {
				s.Logger.Warn("Error getting listening address for UDP connection", "error", err)
				continue
			}
		L:
			for port, listenAddr := range listenAddrs {
				conn, err := s.Binder.ListenPacket("udp", listenAddr, 65536)
				if err != nil {
					s.Logger.Warn("Listen UDP error", "error", err)
					continue L
				}
				s.Logger.Debug(
					"UDP listener",
					"protocol", "udp",
					"bind_addr", syslogConf.BindAddr,
					"port", port,
					"format", syslogConf.Format,
				)
				c <- model.ListenerInfo{
					BindAddr: syslogConf.BindAddr,
					Port:     port,
					Protocol: "udp",
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := s.handleConnection(conn, syslogConf)
					if err != nil && !eerrors.HasFileClosed(err) {
						s.Logger.Warn("UDP connection error", "error", err)
					}
				}()
			}
		}
	}
	close(c)
	wg.Wait()
}

func (s *UdpServiceImpl) handleConnection(conn net.PacketConn, config conf.UDPSourceConfig) (err error) {
	var localPort int
	var path string

	s.AddConnection(conn)
	defer s.RemoveConnection(conn)
	defer s.Logger.Debug("End of UDP connection")

	local := conn.LocalAddr()
	if local != nil {
		l := local.String()
		s := strings.Split(l, ":")
		localPort, err = strconv.Atoi(s[len(s)-1])
		if err != nil {
			path = strings.TrimSpace(l)
		}
	}

	// Syslog UDP server
	for {
		rawmsg, remote, err := model.RawUDPFromConn(conn)
		if err != nil {
			if eerrors.HasFileClosed(err) {
				return io.EOF
			}
			return eerrors.Wrap(err, "Error reading UDP socket")
		}
		if rawmsg.Size == 0 {
			continue
		}
		rawmsg.LocalPort = localPort
		rawmsg.UnixSocketPath = path
		rawmsg.Decoder = config.DecoderBaseConfig
		rawmsg.ConfID = config.ConfID
		rawmsg.Client = ""
		if remote == nil {
			rawmsg.Client = "localhost" // unix socket
		} else {
			rawmsg.Client = strings.Split(remote.String(), ":")[0]
		}
		err = s.rawMessagesQueue.Put(rawmsg)
		if err != nil {
			return eerrors.WithTypes(eerrors.Wrap(err, "Failed to enqueue new raw UDP message"))
		}
		base.CountIncomingMessage(base.UDP, rawmsg.Client, rawmsg.LocalPort, path)
	}
}
