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
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue/tcp"
)

type TcpServerStatus int

const (
	TcpStopped TcpServerStatus = iota
	TcpStarted
)

func initTcpRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type TcpServiceImpl struct {
	StreamingService
	status           TcpServerStatus
	statusChan       chan TcpServerStatus
	reporter         base.Stasher
	rawMessagesQueue *tcp.Ring
	fatalErrorChan   chan struct{}
	fatalOnce        *sync.Once
	parserEnv        *decoders.ParsersEnv
}

func NewTcpService(env *base.ProviderEnv) (*TcpServiceImpl, error) {
	initTcpRegistry()
	s := TcpServiceImpl{
		status:   TcpStopped,
		reporter: env.Reporter,
	}
	s.StreamingService.init()
	s.StreamingService.BaseService.Logger = env.Logger.New("class", "TcpServer")
	s.StreamingService.BaseService.Binder = env.Binder
	s.StreamingService.handler = tcpHandler{Server: &s}
	s.StreamingService.confined = env.Confined
	return &s, nil
}

// Gather asks the TCP service to report metrics
func (s *TcpServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *TcpServiceImpl) Type() base.Types {
	return base.TCP
}

// Start makes the TCP service start
func (s *TcpServiceImpl) Start() ([]model.ListenerInfo, error) {
	s.LockStatus()
	if s.status != TcpStopped {
		s.UnlockStatus()
		return nil, ServerNotStopped
	}
	s.statusChan = make(chan TcpServerStatus, 1)
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}

	// start listening on the required ports
	infos := s.initTCPListeners()
	if len(infos) > 0 {
		s.status = TcpStarted
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := s.Listen()
			if err != nil {
				if eerrors.HasFileClosed(err) {
					s.Logger.Debug("Closed TCP listener", "error", err)
				} else {
					s.Logger.Warn("TCP listen error", "error", err)
				}
			}
		}()
		s.Logger.Info("Listening on TCP", "nb_services", len(infos))
		// start the parsers
		cpus := runtime.NumCPU()
		for i := 0; i < cpus; i++ {
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				err := s.parse()
				if err != nil {
					s.dofatal()
					s.Logger.Error(err.Error())
				}
			}()
		}
	} else {
		s.Logger.Debug("TCP Server not started: no listener")
		close(s.statusChan)
	}
	s.UnlockStatus()
	return infos, nil
}

func (s *TcpServiceImpl) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *TcpServiceImpl) FatalError() chan struct{} {
	return s.fatalErrorChan
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
	s.resetTCPListeners() // close the listeners
	s.CloseConnections()  // close all current connections.
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
func (s *TcpServiceImpl) SetConf(c conf.BaseConfig) {
	s.StreamingService.SetConf(c.TCPSource, c.Parsers, c.Main.InputQueueSize, c.Main.MaxInputMessageSize)
	s.rawMessagesQueue = tcp.NewRing(c.Main.InputQueueSize)
	s.parserEnv = decoders.NewParsersEnv(s.ParserConfigs, s.Logger)
}

func logg(logger log15.Logger, raw *model.RawMessage) log15.Logger {
	// used to avoid to call logger.New in the hot path of parseOne
	return logger.New(
		"protocol", "tcp",
		"client", raw.Client,
		"local_port", raw.LocalPort,
		"unix_socket_path", raw.UnixSocketPath,
		"format", raw.Decoder.Format,
		"confid", raw.ConfID.String(),
	)
}

func (s *TcpServiceImpl) parseOne(raw *model.RawTcpMessage, gen *utils.Generator) error {
	parser, err := s.parserEnv.GetParser(&raw.Decoder)
	if parser == nil || err != nil {
		return decoders.DecodingError(eerrors.Wrapf(err, "Unknown decoder: %s", raw.Decoder.Format))
	}
	defer parser.Release()

	syslogMsgs, err := parser.Parse(raw.Message)
	if err != nil {
		return decoders.DecodingError(eerrors.Wrap(err, "Parsing error"))
	}

	for _, syslogMsg := range syslogMsgs {
		if syslogMsg == nil {
			continue
		}

		full := model.FullFactoryFrom(syslogMsg)
		full.Uid = gen.Uid()
		full.ConfId = raw.ConfID
		full.SourceType = "tcp"
		full.ClientAddr = raw.Client
		full.SourcePath = raw.UnixSocketPath
		full.SourcePort = raw.LocalPort

		err := s.reporter.Stash(full)
		model.FullFree(full)
		if err != nil {
			logg(s.Logger, &raw.RawMessage).Warn("Error stashing TCP message", "error", err)
			if eerrors.IsFatal(err) {
				return eerrors.Wrap(err, "Fatal error pushing TCP message to the Store")
			}
		}
	}
	return nil
}

// parse fetch messages from the raw queue, parse them, and push them to be sent.
func (s *TcpServiceImpl) parse() error {
	gen := utils.NewGenerator()

	for {
		raw, err := s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			return nil
		}
		err = s.parseOne(raw, gen)
		if err != nil {
			base.ParsingErrorCounter.WithLabelValues("tcp", raw.Client, raw.Decoder.Format).Inc()
			logg(s.Logger, &raw.RawMessage).Warn(err.Error())
		}
		model.RawTCPFree(raw)
		if err != nil && eerrors.IsFatal(err) {
			// stop processing when fatal error happens
			return err
		}
	}
}

func makeRawTCPFactory(props tcpProps, confID utils.MyULID, decoder conf.DecoderBaseConfig) func([]byte) *model.RawTcpMessage {
	return func(data []byte) *model.RawTcpMessage {
		raw := model.RawTCPFactory(data)
		raw.Client = props.Client
		raw.LocalPort = props.LocalPort
		raw.UnixSocketPath = props.Path
		raw.ConfID = confID
		raw.Decoder = decoder
		return raw
	}
}

func makeLogger(logger log15.Logger, props tcpProps, protocol string) log15.Logger {
	return logger.New("protocol", protocol, "client", props.Client, "local_port", props.LocalPortStr, "unix_socket_path", props.Path)
}

func clientCounter(props tcpProps, protocol string) {
	base.ClientConnectionCounter.WithLabelValues(protocol, props.Client, props.LocalPortStr, props.Path).Inc()
}

func incomingCounter(props tcpProps, protocol string) {
	base.IncomingMsgsCounter.WithLabelValues(protocol, props.Client, props.LocalPortStr, props.Path).Inc()
}

type tcpHandler struct {
	Server *TcpServiceImpl
}

func (h tcpHandler) HandleConnection(conn net.Conn, config conf.TCPSourceConfig) (err error) {
	s := h.Server
	s.AddConnection(conn)
	props := eprops(conn)
	logger := makeLogger(s.Logger, props, "tcp")
	factory := makeRawTCPFactory(props, config.ConfID, config.DecoderBaseConfig)
	clientCounter(props, "tcp")

	logger.Info("New client")

	defer func() {
		if e := eerrors.Err(recover()); e != nil {
			err = eerrors.Wrap(e, "Scanner panicked in TCP service")
		}
		logger.Debug("Closed connected to TCP Client")
		s.RemoveConnection(conn)
	}()

	timeout := config.Timeout
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, s.MaxMessageSize), s.MaxMessageSize)
	if config.LineFraming {
		scanner.Split(makeLFTCPSplit(config.FrameDelimiter))
	} else {
		scanner.Split(TcpSplit)
	}

	for scanner.Scan() {
		if timeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(timeout))
		}
		buf := scanner.Bytes()
		if len(buf) == 0 {
			continue
		}
		if s.MaxMessageSize > 0 && len(buf) > s.MaxMessageSize {
			return eerrors.Fatal(eerrors.Errorf("Raw TCP message too large: %d > %d", len(buf), s.MaxMessageSize))
		}
		err = s.rawMessagesQueue.Put(factory(buf))
		if err != nil {
			return eerrors.Fatal(eerrors.Wrap(err, "Failed to enqueue new raw TCP message"))
		}
		incomingCounter(props, "tcp")
	}
	err = scanner.Err()
	if eerrors.HasFileClosed(err) {
		return io.EOF
	}
	return eerrors.Wrap(err, "TCP scanning error")
}

func makeLFTCPSplit(delimiter string) func(d []byte, a bool) (int, []byte, error) {
	delim := []byte(delimiter)[0]
	f := func(data []byte, atEOF bool) (advance int, token []byte, eoferr error) {
		if atEOF {
			eoferr = io.EOF
		}
		trimmedData := bytes.TrimLeft(data, " \r\n")
		if len(trimmedData) == 0 {
			return 0, nil, eoferr
		}
		trimmed := len(data) - len(trimmedData)
		lf := bytes.IndexByte(trimmedData, delim)
		if lf < 1 {
			return 0, nil, eoferr
		}
		token = bytes.Trim(trimmedData[0:lf], " \r\n")
		advance = trimmed + lf + 1
		return advance, token, nil
	}
	return f
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

type tcpProps struct {
	LocalPort    int32
	LocalPortStr string
	Client       string
	Path         string
}

func eprops(conn net.Conn) (props tcpProps) {
	remote := conn.RemoteAddr()
	if remote == nil {
		props.Client = "localhost"
		props.LocalPort = 0
		props.Path = conn.LocalAddr().String()
	} else {
		props.Path = ""
		props.Client = strings.Split(remote.String(), ":")[0]
		local := conn.LocalAddr()
		if local != nil {
			s := strings.Split(local.String(), ":")
			props.LocalPort, _ = utils.Atoi32(s[len(s)-1])
		}
	}
	props.LocalPortStr = strconv.FormatInt(int64(props.LocalPort), 10)
	return props
}
