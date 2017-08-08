package server

import (
	"bufio"
	"bytes"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
)

type TcpServerStatus int

const (
	TcpStopped TcpServerStatus = iota
	TcpStarted
)

type TcpServer interface {
	Start() error
	Stop()
	Register(r *consul.Registry)
	Unregister(r *consul.Registry)
	WaitClosed()
	SetConf(c conf.GConfig)
}

type TcpServerPlugin struct {

}

func (s *TcpServerPlugin) Start() error {

}

type tcpServerImpl struct {
	StreamServer
	status     TcpServerStatus
	statusChan chan TcpServerStatus
	stasher    model.Stasher
	metrics    *metrics.Metrics
	generator  chan ulid.ULID
}

func (s *tcpServerImpl) init() {
	s.StreamServer.init()
}

func NewTcpServer(c *conf.GConfig, stasher model.Stasher, gen chan ulid.ULID, b *sys.BinderClient, m *metrics.Metrics, l log15.Logger) TcpServer {
	s := tcpServerImpl{
		status:    TcpStopped,
		stasher:   stasher,
		metrics:   m,
		generator: gen,
	}
	s.logger = l.New("class", "TcpServer")
	s.binder = b
	s.protocol = "tcp"
	s.Conf = *c
	s.init()
	s.handler = tcpHandler{Server: &s}
	return &s
}

func (s *tcpServerImpl) WaitClosed() {
	<-s.statusChan
}

func (s *tcpServerImpl) SetConf(c conf.GConfig) {
	s.Conf = c
}

func (s *tcpServerImpl) Start() error {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != TcpStopped {
		return ServerNotStopped
	}
	s.statusChan = make(chan TcpServerStatus, 1)

	// start listening on the required ports
	nb := s.initTCPListeners()
	if nb > 0 {
		s.status = TcpStarted
		s.Listen()
		s.logger.Info("Listening on TCP", "nb_services", nb)
	} else {
		s.logger.Debug("TCP Server not started: no listening port")
		close(s.statusChan)
	}
	return nil
}

func (s *tcpServerImpl) Stop() {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()
	if s.status != TcpStarted {
		return
	}
	s.resetTCPListeners() // close the listeners. This will make Listen to return and close all current connections.
	s.wg.Wait()           // wait that all HandleConnection goroutines have ended
	s.logger.Debug("TcpServer goroutines have ended")

	s.status = TcpStopped
	s.statusChan <- TcpStopped
	close(s.statusChan)
	s.logger.Debug("TCP server has stopped")
}

type tcpHandler struct {
	Server *tcpServerImpl
}

func (h tcpHandler) HandleConnection(conn net.Conn, config *conf.SyslogConfig) {

	var local_port int

	s := h.Server
	s.AddConnection(conn)

	raw_messages_chan := make(chan *model.RawMessage)

	defer func() {
		close(raw_messages_chan)
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	client := ""
	path := ""
	remote := conn.RemoteAddr()

	if remote == nil {
		client = "localhost"
		local_port = 0
		path = conn.LocalAddr().String()
	} else {
		client = strings.Split(remote.String(), ":")[0]
		local := conn.LocalAddr()
		if local != nil {
			s := strings.Split(local.String(), ":")
			local_port, _ = strconv.Atoi(s[len(s)-1])
		}
	}
	client = strings.TrimSpace(client)
	path = strings.TrimSpace(path)
	local_port_s := strconv.FormatInt(int64(local_port), 10)

	logger := s.logger.New("protocol", s.protocol, "client", client, "local_port", local_port, "unix_socket_path", path, "format", config.Format)
	logger.Info("New client")
	s.metrics.ClientConnectionCounter.WithLabelValues(s.protocol, client, local_port_s, path).Inc()

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		e := NewParsersEnv(s.Conf.Parsers, s.logger)
		for m := range raw_messages_chan {
			parser := e.GetParser(config.Format)
			if parser == nil {
				logger.Error("Unknown parser")
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
				s.metrics.ParsingErrorCounter.WithLabelValues(config.Format, client).Inc()
				logger.Info("Parsing error", "Message", m.Message, "error", err)
			}
		}
	}()

	timeout := config.Timeout
	if timeout > 0 {
		conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner := bufio.NewScanner(conn)
	switch config.Format {
	case "rfc5424", "rfc3164", "json", "auto":
		scanner.Split(TcpSplit)
	default:
		scanner.Split(LFTcpSplit)
	}

	for {
		if scanner.Scan() {
			if timeout > 0 {
				conn.SetReadDeadline(time.Now().Add(timeout))
			}
			raw := model.RawMessage{
				Client:    client,
				LocalPort: local_port,
				Message:   scanner.Text(),
			}
			s.metrics.IncomingMsgsCounter.WithLabelValues(s.protocol, client, local_port_s, path).Inc()
			raw_messages_chan <- &raw
		} else {
			logger.Info("End of TCP client connection", "error", scanner.Err())
			return
		}
	}
}

func LFTcpSplit(data []byte, atEOF bool) (int, []byte, error) {
	trimmed_data := bytes.TrimLeft(data, " \r\n")
	if len(trimmed_data) == 0 {
		return 0, nil, nil
	}
	trimmed := len(data) - len(trimmed_data)
	lf := bytes.IndexByte(trimmed_data, '\n')
	if lf >= 0 {
		token := bytes.Trim(trimmed_data[0:lf], " \r\n")
		advance := trimmed + lf + 1
		return advance, token, nil
	} else {
		// data does not contain a full syslog line
		return 0, nil, nil
	}
}

func TcpSplit(data []byte, atEOF bool) (int, []byte, error) {
	trimmed_data := bytes.TrimLeft(data, " \r\n")
	if len(trimmed_data) == 0 {
		return 0, nil, nil
	}
	trimmed := len(data) - len(trimmed_data)
	if trimmed_data[0] == byte('<') {
		// LF framing
		lf := bytes.IndexByte(trimmed_data, '\n')
		if lf >= 0 {
			token := bytes.Trim(trimmed_data[0:lf], " \r\n")
			advance := trimmed + lf + 1
			return advance, token, nil
		} else {
			// data does not contain a full syslog line
			return 0, nil, nil
		}
	} else {
		// octet counting framing
		sp := bytes.IndexAny(trimmed_data, " \n")
		if sp <= 0 {
			return 0, nil, nil
		}
		datalen_s := bytes.Trim(trimmed_data[0:sp], " \r\n")
		datalen, err := strconv.Atoi(string(datalen_s))
		if err != nil {
			return 0, nil, err
		}
		advance := trimmed + sp + 1 + datalen
		if len(data) >= advance {
			token := bytes.Trim(trimmed_data[sp+1:sp+1+datalen], " \r\n")
			return advance, token, nil
		} else {
			return 0, nil, nil
		}

	}
}
