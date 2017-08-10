package services

import (
	"net"
	"strconv"
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
)

type UdpServerStatus int

const (
	UdpStopped UdpServerStatus = iota
	UdpStarted
)

type udpServiceImpl struct {
	GenericService
	status     UdpServerStatus
	statusChan chan UdpServerStatus
	stasher    model.Stasher
	handler    PacketHandler
	metrics    *metrics.Metrics
	generator  chan ulid.ULID
}

type PacketHandler interface {
	HandleConnection(conn net.PacketConn, config *conf.SyslogConfig)
}

type UdpHandler struct {
	Server *udpServiceImpl
}

func (s *udpServiceImpl) init() {
	s.GenericService.init()
}

func NewUdpService(stasher model.Stasher, gen chan ulid.ULID, b *sys.BinderClient, m *metrics.Metrics, l log15.Logger) NetworkService {
	s := udpServiceImpl{status: UdpStopped, metrics: m, stasher: stasher, generator: gen}
	s.logger = l.New("class", "UdpServer")
	s.binder = b
	s.init()
	s.protocol = "udp"
	s.handler = UdpHandler{Server: &s}
	return &s
}

func (s *udpServiceImpl) handleConnection(conn net.PacketConn, config *conf.SyslogConfig) {
	s.handler.HandleConnection(conn, config)
}

func (s *udpServiceImpl) SetKafkaConf(kc *conf.KafkaConfig) {}

func (s *udpServiceImpl) Start(test bool) ([]*model.ListenerInfo, error) {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != UdpStopped {
		return nil, ServerNotStopped
	}
	s.statusChan = make(chan UdpServerStatus, 1)

	s.connections = map[Connection]bool{}
	infos := s.ListenPacket()
	if len(infos) > 0 {
		s.status = UdpStarted
		s.logger.Info("Listening on UDP", "nb_services", len(infos))
	} else {
		s.logger.Debug("The UDP service has not been started: no listening port")
		close(s.statusChan)
	}
	return infos, nil
}

func (s *udpServiceImpl) Stop() {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != UdpStarted {
		return
	}
	s.logger.Debug("Closing UDP connections")
	s.CloseConnections()
	s.logger.Debug("Waiting for UDP goroutines")
	s.wg.Wait()
	s.logger.Debug("UdpServer goroutines have ended")

	s.status = UdpStopped
	s.statusChan <- UdpStopped
	close(s.statusChan)
	s.logger.Debug("Udp server has stopped")
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
	s.unixSocketPaths = []string{}
	for _, syslogConf := range s.SyslogConfigs {
		if syslogConf.Protocol == "udp" {
			if len(syslogConf.UnixSocketPath) > 0 {
				conn, err := net.ListenPacket("unixgram", syslogConf.UnixSocketPath)
				if err != nil {
					switch err.(type) {
					case *net.OpError:
						if s.binder == nil {
							s.logger.Warn("Listen unixgram OpError", "error", err)
							conn = nil
						} else {
							s.logger.Info("Listen unixgram OpError. Retrying as root.", "error", err)
							conn, err = s.binder.ListenPacket("unixgram", syslogConf.UnixSocketPath)
							if err != nil {
								s.logger.Warn("Listen unixgram OpError", "error", err)
								conn = nil
							}
						}
					default:
						s.logger.Warn("Listen unixgram error", "error", err)
						conn = nil
					}
				}
				if conn != nil && err == nil {
					s.logger.Debug("Listener", "protocol", s.protocol, "path", syslogConf.UnixSocketPath, "format", syslogConf.Format)
					udpinfos = append(udpinfos, &model.ListenerInfo{
						UnixSocketPath: syslogConf.UnixSocketPath,
						Protocol:       s.protocol,
					})
					s.unixSocketPaths = append(s.unixSocketPaths, syslogConf.UnixSocketPath)
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf)
				}
			} else {
				listenAddr, _ := syslogConf.GetListenAddr()
				conn, err := net.ListenPacket("udp", listenAddr)
				if err != nil {
					switch err.(type) {
					case *net.OpError:
						if s.binder == nil || syslogConf.Port > 1024 {
							s.logger.Warn("Listen UDP OpError", "error", err)
							conn = nil
						} else {
							s.logger.Info("Listen unixgram OpError. Retrying as root.", "error", err)
							conn, err = s.binder.ListenPacket("udp", listenAddr)
							if err != nil {
								s.logger.Warn("Listen UDP OpError", "error", err)
								conn = nil
							}
						}
					default:
						s.logger.Warn("Listen UDP error", "error", err)
						conn = nil
					}

				}
				if conn != nil && err == nil {
					s.logger.Debug("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
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

	logger := s.logger.New("protocol", s.protocol, "local_port", local_port, "unix_socket_path", path, "format", config.Format)

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		e := NewParsersEnv(s.ParserConfigs, s.logger)
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
				s.metrics.ParsingErrorCounter.WithLabelValues(config.Format, m.Client).Inc()
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
		s.metrics.IncomingMsgsCounter.WithLabelValues(s.protocol, client, local_port_s, path).Inc()
		raw_messages_chan <- &raw
	}

}
