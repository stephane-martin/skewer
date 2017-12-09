package network

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

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
	"github.com/stephane-martin/skewer/utils/queue/tcp"
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
	m := tcpMetrics{
		IncomingMsgsCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_incoming_messages_total",
				Help: "total number of messages that were received",
			},
			[]string{"protocol", "client", "port", "path"},
		),

		ClientConnectionCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_client_connections_total",
				Help: "total number of client connections",
			},
			[]string{"protocol", "client", "port", "path"},
		),
		ParsingErrorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_parsing_errors_total",
				Help: "total number of times there was a parsing error",
			},
			[]string{"protocol", "client", "parser_name"},
		),
	}
	return &m
}

type TcpServiceImpl struct {
	StreamingService
	status           TcpServerStatus
	statusChan       chan TcpServerStatus
	reporter         *base.Reporter
	generator        chan ulid.ULID
	metrics          *tcpMetrics
	registry         *prometheus.Registry
	rawMessagesQueue *tcp.Ring
}

func NewTcpService(reporter *base.Reporter, gen chan ulid.ULID, b *binder.BinderClient, l log15.Logger) *TcpServiceImpl {
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
	s.StreamingService.handler = tcpHandler{Server: &s}
	return &s
}

// Gather asks the TCP service to report metrics
func (s *TcpServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

// Start makes the TCP service start
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
		// start the parsers
		cpus := runtime.NumCPU()
		for i := 0; i < cpus; i++ {
			s.wg.Add(1)
			go s.Parse()
		}
	} else {
		s.Logger.Debug("TCP Server not started: no listener")
		close(s.statusChan)
	}
	s.UnlockStatus()
	return infos, nil
}

// Shutdown is just Stop for the TCP service
func (s *TcpServiceImpl) Shutdown() {
	s.Stop()
}

// Stop makes the TCP service stop
func (s *TcpServiceImpl) Stop() {
	s.LockStatus()
	if s.status != TcpStarted {
		s.UnlockStatus()
		return
	}
	s.resetTCPListeners() // close the listeners. This will make Listen to return and close all current connections.
	if s.rawMessagesQueue != nil {
		s.rawMessagesQueue.Dispose()
	}
	s.wg.Wait() // wait that all goroutines have ended
	s.Logger.Debug("TcpServer goroutines have ended")

	s.status = TcpStopped
	s.statusChan <- TcpStopped
	close(s.statusChan)
	s.Logger.Debug("TCP server has stopped")
	s.UnlockStatus()
}

// SetConf configures the TCP service
func (s *TcpServiceImpl) SetConf(sc []conf.TcpSourceConfig, pc []conf.ParserConfig, queueSize uint64, messageSize int) {
	s.BaseService.Pool = &sync.Pool{New: func() interface{} {
		return &model.RawTcpMessage{Message: make([]byte, messageSize)}
	}}
	s.StreamingService.SetConf(sc, pc, queueSize, messageSize)
	s.rawMessagesQueue = tcp.NewRing(queueSize)
}

func (s *TcpServiceImpl) ParseOne(env *ParsersEnv, raw *model.RawTcpMessage) {
	// be sure to free the raw pointer
	defer s.Pool.Put(raw)

	logger := s.Logger.New(
		"protocol", "tcp",
		"client", raw.Client,
		"local_port", raw.LocalPort,
		"unix_socket_path", raw.UnixSocketPath,
		"format", raw.Format,
	)

	decoder := utils.SelectDecoder(raw.Encoding)
	parser := env.GetParser(raw.Format)

	if parser == nil {
		logger.Error("Unknown parser")
		return
	}

	syslogMsg, err := parser.Parse(raw.Message[:raw.Size], decoder, raw.DontParseSD)
	if err != nil {
		s.metrics.ParsingErrorCounter.WithLabelValues("tcp", raw.Client, raw.Format).Inc()
		logger.Info("Parsing error", "Message", raw.Message, "error", err)
		return
	}
	if syslogMsg == nil {
		return
	}

	fatal, nonfatal := s.reporter.Stash(model.FullMessage{
		Parsed: model.ParsedMessage{
			Fields:         *syslogMsg,
			Client:         raw.Client,
			LocalPort:      raw.LocalPort,
			UnixSocketPath: raw.UnixSocketPath,
		},
		Uid:    <-s.generator,
		ConfId: raw.ConfID,
	})

	if fatal != nil {
		logger.Error("Fatal error stashing TCP message", "error", fatal)
		// TODO: shutdown
	} else if nonfatal != nil {
		logger.Warn("Non-fatal error stashing TCP message", "error", nonfatal)
	}
}

// Parse fetch messages from the raw queue, parse them, and push them to be sent.
func (s *TcpServiceImpl) Parse() {
	defer s.wg.Done()

	env := NewParsersEnv(s.ParserConfigs, s.Logger)

	for {
		raw, err := s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			break
		}
		s.ParseOne(env, raw)
	}
}

type tcpHandler struct {
	Server *TcpServiceImpl
}

func (h tcpHandler) HandleConnection(conn net.Conn, config conf.TcpSourceConfig) {

	s := h.Server
	s.AddConnection(conn)

	defer func() {
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	client := ""
	path := ""
	remote := conn.RemoteAddr()
	localPort := ""
	var localPortInt int

	if remote == nil {
		client = "localhost"
		path = conn.LocalAddr().String()
	} else {
		client = strings.Split(remote.String(), ":")[0]
		local := conn.LocalAddr()
		if local != nil {
			s := strings.Split(local.String(), ":")
			localPortInt, _ = strconv.Atoi(s[len(s)-1])
			if localPortInt > 0 {
				localPort = strconv.FormatInt(int64(localPortInt), 10)
			}
		}
	}
	client = strings.TrimSpace(client)
	path = strings.TrimSpace(path)

	logger := s.Logger.New("protocol", "tcp", "client", client, "local_port", localPort, "unix_socket_path", path, "format", config.Format)
	logger.Info("New client")
	s.metrics.ClientConnectionCounter.WithLabelValues("tcp", client, localPort, path).Inc()

	timeout := config.Timeout
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, s.MaxMessageSize), s.MaxMessageSize)
	switch config.Format {
	case "rfc5424", "rfc3164", "json", "auto":
		scanner.Split(TcpSplit)
	default:
		scanner.Split(lfTCPSplit)
	}
	var rawmsg *model.RawTcpMessage
	var buf []byte

	for scanner.Scan() {
		if timeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(timeout))
		}
		buf = scanner.Bytes()
		if len(buf) == 0 {
			continue
		}
		rawmsg = s.Pool.Get().(*model.RawTcpMessage)
		rawmsg.Client = client
		rawmsg.LocalPort = localPortInt
		rawmsg.UnixSocketPath = path
		rawmsg.Size = len(buf)
		rawmsg.ConfID = config.ConfID
		rawmsg.DontParseSD = config.DontParseSD
		rawmsg.Encoding = config.Encoding
		rawmsg.Format = config.Format
		copy(rawmsg.Message, buf)
		err := s.rawMessagesQueue.Put(rawmsg)
		if err != nil {
			// rawMessagesQueue has been disposed
			logger.Warn("Error queueing TCP raw message", "error", err)
			return
		}
		s.metrics.IncomingMsgsCounter.WithLabelValues("tcp", client, localPort, path).Inc()
	}
	logger.Info("End of TCP client connection", "error", scanner.Err())
}

func lfTCPSplit(data []byte, atEOF bool) (advance int, token []byte, eoferr error) {
	if atEOF {
		eoferr = io.EOF
	}
	trimmedData := bytes.TrimLeft(data, " \r\n")
	if len(trimmedData) == 0 {
		return 0, nil, eoferr
	}
	trimmed := len(data) - len(trimmedData)
	lf := bytes.IndexByte(trimmedData, '\n')
	if lf == 0 {
		return 0, nil, eoferr
	}
	token = bytes.Trim(trimmedData[0:lf], " \r\n")
	advance = trimmed + lf + 1
	return advance, token, nil
}

func getline(data []byte, trimmed int, eoferr error) (int, []byte, error) {
	lf := bytes.IndexByte(data, '\n')
	if lf == 0 {
		return 0, nil, eoferr
	}
	token := bytes.Trim(data[0:lf], " \r\n")
	return lf + trimmed + 1, token, nil
}

func TcpSplit(data []byte, atEOF bool) (advance int, token []byte, eoferr error) {
	if atEOF {
		eoferr = io.EOF
	}
	trimmedData := bytes.TrimLeft(data, " \r\n")
	if len(trimmedData) == 0 {
		return 0, nil, eoferr
	}
	trimmed := len(data) - len(trimmedData)
	if trimmedData[0] == byte('<') {
		return getline(trimmedData, trimmed, eoferr)
	}
	// octet counting framing?
	sp := bytes.IndexAny(trimmedData, " \n")
	if sp <= 0 {
		return 0, nil, eoferr
	}
	datalenStr := bytes.Trim(trimmedData[0:sp], " \r\n")
	datalen, err := strconv.Atoi(string(datalenStr))
	if err != nil {
		// the first part is not a number, so back to LF
		return getline(trimmedData, trimmed, eoferr)
	}
	advance = trimmed + sp + 1 + datalen
	if len(data) < advance {
		return 0, nil, eoferr
	}
	token = bytes.Trim(trimmedData[sp+1:sp+1+datalen], " \r\n")
	return advance, token, nil

}
