package server

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/metrics"
	"github.com/stephane-martin/relp2kafka/model"
	"github.com/stephane-martin/relp2kafka/store"
	"github.com/stephane-martin/relp2kafka/sys"
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
	store      *store.MessageStore
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

func NewUdpServer(c *conf.GConfig, st *store.MessageStore, generator chan ulid.ULID, metrics *metrics.Metrics, logger log15.Logger) *UdpServer {
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
	} else {
		s.logger.Info("The UDP service has not been started: no listening port")
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
	s.logger.Info("Udp server has stopped")
}

func (s *UdpServer) ListenPacket() int {
	s.unixSocketPaths = []string{}
	nb := 0
	for _, syslogConf := range s.Conf.Syslog {
		if syslogConf.Protocol == "udp" {
			confId, err := s.store.StoreSyslogConfig(&syslogConf)
			if err != nil {
				// todo: log
				continue
			}
			if len(syslogConf.UnixSocketPath) > 0 {
				addr, _ := net.ResolveUnixAddr("unixgram", syslogConf.UnixSocketPath)
				conn, err := net.ListenUnixgram("unixgram", addr)
				if err != nil {
					s.logger.Warn("Error listening on datagram unix socket", "path", syslogConf.UnixSocketPath, "error", err)
				} else if conn != nil {
					s.logger.Info("Listener", "protocol", s.protocol, "path", syslogConf.UnixSocketPath, "format", syslogConf.Format)
					nb++
					s.unixSocketPaths = append(s.unixSocketPaths, syslogConf.UnixSocketPath)
					s.wg.Add(1)
					go s.handleConnection(conn, syslogConf, confId)
				}
			} else {
				listenAddr, _ := syslogConf.GetListenAddr()
				conn, err := net.ListenPacket("udp", listenAddr)
				if err != nil {
					s.logger.Warn("Error listening on UDP", "addr", listenAddr, "error", err)
				} else if conn != nil {
					s.logger.Info("Listener", "protocol", s.protocol, "bind_addr", syslogConf.BindAddr, "port", syslogConf.Port, "format", syslogConf.Format)
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

	if unixConn, ok := conn.(*net.UnixConn); ok {
		fmt.Println("UnixConn!")
		a, b, c, e := sys.GetCredentials(unixConn)
		if e == nil {
			fmt.Println(a, b, c)
		} else {
			fmt.Println(e)
		}
	}

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

	logger := s.logger.New("protocol", s.protocol, "local_port", local_port, "unix_socket_path", path)

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		e := s.NewParsersEnv()
		for m := range raw_messages_chan {
			parser := e.GetParser(config.Format)
			if parser == nil {
				s.logger.Error("Unknown parser", "client", m.Client, "local_port", m.LocalPort, "path", m.UnixSocketPath, "format", config.Format)
				continue
			}
			p, err := parser.Parse(m.Message, config.DontParseSD)

			if err == nil {
				uid := <-s.generator
				parsed_msg := model.TcpUdpParsedMessage{
					Parsed: model.ParsedMessage{
						Fields:         p,
						Client:         m.Client,
						LocalPort:      m.LocalPort,
						UnixSocketPath: m.UnixSocketPath,
					},
					Uid:    uid.String(),
					ConfId: confId,
				}
				s.store.Inputs <- &parsed_msg
			} else {
				logger.Info("Parsing error", "Message", m.Message, "error", err)
			}
		}
	}()

	// Syslog UDP server
	for {
		packet := make([]byte, 65536)
		size, remote, err := conn.ReadFrom(packet)
		if err != nil {
			logger.Info("Error reading UDP", "error", err)
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
