package server

import (
	"net"
	"strconv"
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/store"
)

type UdpServerStatus int

const (
	UdpStopped UdpServerStatus = iota
	UdpStarted
)

type UdpServer struct {
	Server
	status     UdpServerStatus
	ClosedChan chan UdpServerStatus
	store      store.Store
	handler    PacketHandler
	metrics    *metrics.Metrics
	generator  chan ulid.ULID
}

type PacketHandler interface {
	HandleConnection(conn net.PacketConn, config conf.SyslogConfig, confId string)
}

type UdpHandler struct {
	Server *UdpServer
}

func (s *UdpServer) init() {
	s.Server.init()
}

func NewUdpServer(c *conf.GConfig, st store.Store, generator chan ulid.ULID, metrics *metrics.Metrics, logger log15.Logger) *UdpServer {
	s := UdpServer{status: UdpStopped, metrics: metrics, store: st, generator: generator}
	s.logger = logger.New("class", "UdpServer")
	s.init()
	s.protocol = "udp"
	s.Conf = *c
	s.handler = UdpHandler{Server: &s}

	return &s
}

func (s *UdpServer) handleConnection(conn net.PacketConn, config conf.SyslogConfig, confId string) {
	s.handler.HandleConnection(conn, config, confId)
}

func (s *UdpServer) Start() (err error) {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != UdpStopped {
		err = ServerNotStopped
		return
	}
	s.ClosedChan = make(chan UdpServerStatus, 1)

	s.connections = map[Connection]bool{}
	nb := s.ListenPacket()
	if nb > 0 {
		s.status = UdpStarted
		s.logger.Info("Listening on UDP", "nb_services", nb)
	} else {
		s.logger.Debug("The UDP service has not been started: no listening port")
		close(s.ClosedChan)
	}
	return
}

func (s *UdpServer) Stop() {
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
	s.ClosedChan <- UdpStopped
	close(s.ClosedChan)
	s.logger.Debug("Udp server has stopped")
}

func (s *UdpServer) ListenPacket() int {
	s.unixSocketPaths = []string{}
	nb := 0
	for _, syslogConf := range s.Conf.Syslog {
		if syslogConf.Protocol == "udp" {
			confId, err := s.store.StoreSyslogConfig(&syslogConf)
			if err != nil {
				s.logger.Error("Error persisting the syslog configuration to the Store", "error", err)
				continue
			}
			if len(syslogConf.UnixSocketPath) > 0 {
				addr, _ := net.ResolveUnixAddr("unixgram", syslogConf.UnixSocketPath)
				conn, err := net.ListenUnixgram("unixgram", addr)
				if err != nil {
					switch err.(type) {
					case *net.OpError:
						s.logger.Info("Listen unixgram OpError", "error", err)
					default:
						s.logger.Warn("Listen unixgram error", "error", err)
					}
				} else if conn != nil {
					s.logger.Debug("Listener", "protocol", s.protocol, "path", syslogConf.UnixSocketPath, "format", syslogConf.Format)
					nb++
					s.unixSocketPaths = append(s.unixSocketPaths, syslogConf.UnixSocketPath)
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf, confId)
				}
			} else {
				listenAddr, _ := syslogConf.GetListenAddr()
				conn, err := net.ListenPacket("udp", listenAddr)
				if err != nil {
					switch err.(type) {
					case *net.OpError:
						s.logger.Info("Listen UDP OpError", "error", err)
					default:
						s.logger.Warn("Listen UDP error", "error", err)
					}

				} else if conn != nil {
					s.logger.Debug("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
					nb++
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf, confId)
				}
			}
		}
	}
	return nb
}

func (h UdpHandler) HandleConnection(conn net.PacketConn, config conf.SyslogConfig, confId string) {
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

	inputs := s.store.Inputs()

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		e := NewParsersEnv(s.Conf.Parsers, s.logger)
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
					ConfId: confId,
				}
				inputs <- &parsed_msg
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
