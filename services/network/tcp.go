package network

import (
	"bufio"
	"bytes"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/services/errors"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

type TcpServerStatus int

const (
	TcpStopped TcpServerStatus = iota
	TcpStarted
)

type tcpMetrics struct {
	ClientConnectionCounter *prometheus.CounterVec
	IncomingMsgsCounter     *prometheus.CounterVec
	ParsingErrorCounter     *prometheus.CounterVec
}

func NewTcpMetrics() *tcpMetrics {
	m := &tcpMetrics{}
	m.IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_incoming_messages_total",
			Help: "total number of syslog messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)
	m.ClientConnectionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_client_connections_total",
			Help: "total number of client connections",
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

type TcpServiceImpl struct {
	StreamingService
	status     TcpServerStatus
	statusChan chan TcpServerStatus
	reporter   *base.Reporter
	generator  chan ulid.ULID
	metrics    *tcpMetrics
	registry   *prometheus.Registry
}

func NewTcpService(reporter *base.Reporter, gen chan ulid.ULID, b *sys.BinderClient, l log15.Logger) *TcpServiceImpl {
	s := TcpServiceImpl{
		status:    TcpStopped,
		reporter:  reporter,
		generator: gen,
		metrics:   NewTcpMetrics(),
		registry:  prometheus.NewRegistry(),
	}
	s.StreamingService.init()
	s.registry.MustRegister(s.metrics.ClientConnectionCounter, s.metrics.IncomingMsgsCounter, s.metrics.ParsingErrorCounter)
	s.StreamingService.BaseService.Logger = l.New("class", "TcpServer")
	s.StreamingService.BaseService.Binder = b
	s.StreamingService.BaseService.Protocol = "tcp"
	s.StreamingService.handler = tcpHandler{Server: &s}
	return &s
}

func (s *TcpServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *TcpServiceImpl) Start(test bool) ([]model.ListenerInfo, error) {
	s.LockStatus()
	if s.status != TcpStopped {
		s.UnlockStatus()
		return nil, errors.ServerNotStopped
	}
	s.statusChan = make(chan TcpServerStatus, 1)

	// start listening on the required ports
	infos := s.initTCPListeners()
	if len(infos) > 0 {
		s.status = TcpStarted
		s.Listen()
		s.Logger.Info("Listening on TCP", "nb_services", len(infos))
	} else {
		s.Logger.Debug("TCP Server not started: no listener")
		close(s.statusChan)
	}
	s.UnlockStatus()
	return infos, nil
}

func (s *TcpServiceImpl) Shutdown() {
	s.Stop()
}

func (s *TcpServiceImpl) Stop() {
	s.LockStatus()
	if s.status != TcpStarted {
		s.UnlockStatus()
		return
	}
	s.resetTCPListeners() // close the listeners. This will make Listen to return and close all current connections.
	s.wg.Wait()           // wait that all HandleConnection goroutines have ended
	s.Logger.Debug("TcpServer goroutines have ended")

	s.status = TcpStopped
	s.statusChan <- TcpStopped
	close(s.statusChan)
	s.Logger.Debug("TCP server has stopped")
	s.UnlockStatus()
}

type tcpHandler struct {
	Server *TcpServiceImpl
}

func (h tcpHandler) HandleConnection(conn net.Conn, config *conf.SyslogConfig) {

	var local_port int

	s := h.Server
	s.AddConnection(conn)

	raw_messages_chan := make(chan model.RawMessage)

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

	logger := s.Logger.New("protocol", s.Protocol, "client", client, "local_port", local_port, "unix_socket_path", path, "format", config.Format)
	logger.Info("New client")
	if s.metrics != nil {
		s.metrics.ClientConnectionCounter.WithLabelValues(s.Protocol, client, local_port_s, path).Inc()
	}

	// pull messages from raw_messages_chan, parse them and push them to the Store
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		e := NewParsersEnv(s.ParserConfigs, s.Logger)
		parser := e.GetParser(config.Format)
		if parser == nil {
			logger.Crit("Unknown parser")
			return
		}

		var uid ulid.ULID
		var syslogMsg *model.SyslogMessage
		var err error
		var fullMsg model.TcpUdpParsedMessage
		var raw model.RawMessage
		decoder := utils.SelectDecoder(config.Encoding)

		for raw = range raw_messages_chan {

			syslogMsg, err = parser.Parse(raw.Message, decoder, config.DontParseSD)

			if err == nil && syslogMsg != nil {
				uid = <-s.generator
				fullMsg = model.TcpUdpParsedMessage{
					Parsed: model.ParsedMessage{
						Fields:         *syslogMsg,
						Client:         raw.Client,
						LocalPort:      raw.LocalPort,
						UnixSocketPath: raw.UnixSocketPath,
					},
					Uid:    uid.String(),
					ConfId: config.ConfID,
				}
				fatal, nonfatal := s.reporter.Stash(fullMsg)
				if fatal != nil {
					// TODO: the Store is not working properly
				} else if nonfatal != nil {
					logger.Warn("Error stashing TCP message", "error", nonfatal)
				}
			} else {
				if s.metrics != nil {
					s.metrics.ParsingErrorCounter.WithLabelValues(s.Protocol, client, config.Format).Inc()
				}
				logger.Info("Parsing error", "Message", raw.Message, "error", err)
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

	var raw model.RawMessage
	for {
		if scanner.Scan() {
			if timeout > 0 {
				conn.SetReadDeadline(time.Now().Add(timeout))
			}
			raw = model.RawMessage{
				Client:    client,
				LocalPort: local_port,
				Message:   scanner.Bytes(),
			}
			if s.metrics != nil {
				s.metrics.IncomingMsgsCounter.WithLabelValues(s.Protocol, client, local_port_s, path).Inc()
			}
			raw_messages_chan <- raw
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
