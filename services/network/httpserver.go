package network

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue/tcp"
	"github.com/valyala/bytebufferpool"
	"go.uber.org/atomic"
)

var requestBodyPool bytebufferpool.Pool

var copyBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 65536)
	},
}

func copyZeroAlloc(w io.Writer, r io.Reader) (int64, error) {
	buf := copyBufPool.Get().([]byte)
	n, err := io.CopyBuffer(w, r, buf)
	copyBufPool.Put(buf)
	return n, err
}

func initHTTPRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

func newTracker(count int64, callbackOK func(), callbackFail func()) *requestTracker {
	r := requestTracker{
		connID:       utils.NewUid(),
		callbackOK:   callbackOK,
		callbackFail: callbackFail,
	}
	r.count.Store(count)
	r.Lock()
	return &r
}

type requestTracker struct {
	count        atomic.Int64
	callbackOK   func()
	callbackFail func()
	connID       utils.MyULID
	failed       atomic.Bool
	sync.Mutex
	sync.Once
}

func (r *requestTracker) done() {
	if r.count.Dec() == 0 && !r.failed.Load() {
		r.finish(true)
	}
}

func (r *requestTracker) finish(ok bool) {
	r.Do(func() {
		if ok {
			r.callbackOK()
		} else {
			r.callbackFail()
		}
		r.Unlock()
	})
}

func (r *requestTracker) fail() {
	r.count.Dec()
	if r.failed.CAS(false, true) {
		r.finish(false)
	}
}

func (r *requestTracker) cancel() {
	r.finish(false)
}

func (r *requestTracker) wait() {
	r.Lock()
	r.Unlock()
}

type HTTPServiceImpl struct {
	configs          []conf.HTTPServerSourceConfig
	parserConfigs    []conf.ParserConfig
	parserEnv        *decoders.ParsersEnv
	reporter         base.Stasher
	rawMessagesQueue *tcp.Ring
	maxMessageSize   int
	logger           log15.Logger
	binder           binder.Client
	wg               sync.WaitGroup
	stopCtx          context.Context
	stop             context.CancelFunc
	rawpool          *sync.Pool
	fatalErrorChan   chan struct{}
	fatalOnce        *sync.Once
	confined         bool
	trackers         *sync.Map
}

func NewHTTPService(env *base.ProviderEnv) (base.Provider, error) {
	initHTTPRegistry()
	s := HTTPServiceImpl{
		reporter: env.Reporter,
		logger:   env.Logger.New("class", "HTTPService"),
		binder:   env.Binder,
		confined: env.Confined,
	}
	return &s, nil
}

func (s *HTTPServiceImpl) Type() base.Types {
	return base.HTTPServer
}

func (s *HTTPServiceImpl) addTracker(count int64, callbackOK func(), callbackFail func()) *requestTracker {
	tracker := newTracker(count, callbackOK, callbackFail)
	s.trackers.Store(tracker.connID, tracker)
	return tracker
}

func (s *HTTPServiceImpl) removeTracker(connID utils.MyULID) {
	if t, ok := s.trackers.Load(connID); ok {
		s.trackers.Delete(connID)
		t.(*requestTracker).cancel()
	}
}

func (s *HTTPServiceImpl) done(connID utils.MyULID) {
	if t, ok := s.trackers.Load(connID); ok {
		t.(*requestTracker).done()
	}
}

func (s *HTTPServiceImpl) fail(connID utils.MyULID) {
	if t, ok := s.trackers.Load(connID); ok {
		t.(*requestTracker).fail()
	}
}

func (s *HTTPServiceImpl) SetConf(c conf.BaseConfig) {
	s.rawpool = &sync.Pool{New: func() interface{} {
		return &model.RawTcpMessage{Message: make([]byte, c.Main.MaxInputMessageSize)}
	}}
	s.maxMessageSize = c.Main.MaxInputMessageSize
	s.configs = c.HTTPServerSource
	s.parserConfigs = c.Parsers
	s.parserEnv = decoders.NewParsersEnv(s.parserConfigs, s.logger)
	s.rawMessagesQueue = tcp.NewRing(c.Main.InputQueueSize)
	s.trackers = &sync.Map{}
}

func (s *HTTPServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *HTTPServiceImpl) Start() (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	s.stopCtx, s.stop = context.WithCancel(context.Background())
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}
	for _, config := range s.configs {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := s.startOne(config)
			if err != nil {
				if isSetupError(err) {
					s.logger.Error("Error setting up the HTTP service", "error", err)
				} else {
					s.logger.Error("Error running the HTTP service", "error", err)
				}
				s.dofatal()
			}
		}()
	}
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := s.parse()
			if err != nil {
				s.logger.Error("Fatal error processing messages", "error", err)
				s.dofatal()
			}
		}()
	}

	return infos, nil
}

func (s *HTTPServiceImpl) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *HTTPServiceImpl) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func getListener(bindr binder.Client, addr string, port int, keepalive bool, period time.Duration) (listener net.Listener, err error) {
	hostport := net.JoinHostPort(addr, strconv.FormatInt(int64(port), 10))
	if keepalive {
		listener, err = bindr.ListenKeepAlive("tcp", hostport, period)
	} else {
		listener, err = bindr.Listen("tcp", hostport)
	}
	if err != nil {
		return nil, err
	}
	return listener, nil
}

func setupError(err error) error {
	return eerrors.WithTypes(err, "Setup")
}

func isSetupError(err error) bool {
	return eerrors.Is("Setup", err)
}

func (s *HTTPServiceImpl) startOne(config conf.HTTPServerSourceConfig) error {
	server := &http.Server{
		Handler:           http.HandlerFunc(s.handler(config)),
		ReadTimeout:       config.ReadTimeout,
		ReadHeaderTimeout: config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
		ErrorLog:          log.New(s, "", 0),
	}

	server.SetKeepAlivesEnabled(!config.DisableHTTPKeepAlive)

	var serve func() error

	if config.TLSEnabled {
		tlsConf, err := utils.NewTLSConfig("", config.CAFile, config.CAPath, config.CertFile, config.KeyFile, false, s.confined)
		if err != nil {
			return setupError(eerrors.Wrap(err, "Error setting up TLS configuration"))
		}
		tlsConf.ClientAuth = config.GetClientAuthType()
		server.TLSConfig = tlsConf
		listener, err := getListener(s.binder, config.BindAddr, config.Port, !config.DisableConnKeepAlive, config.ConnKeepAlivePeriod)
		if err != nil {
			return setupError(eerrors.Wrap(err, "Error creating TCP listener"))
		}
		defer listener.Close()
		serve = func() error { return server.ServeTLS(listener, "", "") }
	} else {
		listener, err := getListener(s.binder, config.BindAddr, config.Port, !config.DisableConnKeepAlive, config.ConnKeepAlivePeriod)
		if err != nil {
			return setupError(eerrors.Wrap(err, "Error creating TCP listener"))
		}
		defer listener.Close()
		serve = func() error { return server.Serve(listener) }
	}

	// close the server when stopChan is closed
	// this will make the serve() call to return
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-s.stopCtx.Done()
		server.Close()
	}()

	err := serve()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func getBody(reader io.ReadCloser, w http.ResponseWriter, maxSize int64) (buf *bytebufferpool.ByteBuffer, err error) {
	buf = requestBodyPool.Get()
	buf.Reset()
	if maxSize > 0 {
		reader = http.MaxBytesReader(w, reader, maxSize)
	}
	_, err = copyZeroAlloc(buf, reader)
	reader.Close()
	if err != nil {
		requestBodyPool.Put(buf)
		return nil, err
	}
	return buf, nil
}

func releaseBody(buf **bytebufferpool.ByteBuffer) {
	if *buf != nil {
		requestBodyPool.Put(*buf)
		*buf = nil
	}
}

func (s *HTTPServiceImpl) handler(config conf.HTTPServerSourceConfig) func(http.ResponseWriter, *http.Request) {
	ports := strconv.FormatInt(int64(config.Port), 10)
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBuf, err := getBody(r.Body, w, config.MaxBodySize)
		if err != nil {
			s.logger.Warn("Error reading request body", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// ensure that we give back the buffer. releaseBody can be safely called multiple times.
		defer releaseBody(&bodyBuf)
		if r.Method != "POST" {
			s.logger.Warn("Request method is not POST", "method", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if config.DisableMultiple {
			// the body should contain only one message

			tmp := bytes.TrimSpace(bodyBuf.Bytes())
			if len(tmp) == 0 {
				s.logger.Warn("Request did not contain any message")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if s.maxMessageSize > 0 && len(tmp) > s.maxMessageSize {
				s.logger.Warn("Request contains a too large message")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			tracker := s.addTracker(1, func() { w.WriteHeader(http.StatusCreated) }, func() { w.WriteHeader(http.StatusBadRequest) })
			defer s.removeTracker(tracker.connID)

			raw := s.rawpool.Get().(*model.RawTcpMessage)
			raw.Message = raw.Message[:len(tmp)]
			// we *copy* tmp so that we can safely release bodyBuf afterwards
			copy(raw.Message, tmp)
			releaseBody(&bodyBuf)

			raw.Decoder = config.DecoderBaseConfig
			raw.Client = r.RemoteAddr
			raw.ConfID = config.ConfID
			raw.LocalPort = int32(config.Port)
			raw.ConnID = tracker.connID

			s.rawMessagesQueue.Put(raw)
			base.IncomingMsgsCounter.WithLabelValues("httpserver", raw.Client, ports, "").Inc()

			tracker.wait()
			return
		}

		// multiple messages may be present in the body
		// we assume they are separated by config.FrameDelimiter
		tmp := bytes.Split(bodyBuf.Bytes(), []byte(config.FrameDelimiter))
		byteMsgs := make([][]byte, 0, len(tmp))
		var byteMsg []byte
		var trim func([]byte) []byte
		switch config.FrameDelimiter {
		case " ", "\n", "\r", "\r\n":
			trim = bytes.TrimSpace
		default:
			trim = func(b []byte) []byte { return bytes.TrimSpace(bytes.Trim(b, config.FrameDelimiter)) }
		}

		for _, byteMsg = range tmp {
			byteMsg = trim(byteMsg)
			if len(byteMsg) > 0 {
				byteMsgs = append(byteMsgs, byteMsg)
			}
			if s.maxMessageSize > 0 && len(byteMsg) > s.maxMessageSize {
				s.logger.Warn("Request contains a too large message")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		if len(byteMsgs) == 0 {
			s.logger.Debug("Request did not contain any message")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if config.MaxMessages > 0 && len(byteMsg) > config.MaxMessages {
			s.logger.Debug("Request contains too many messages")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tracker := s.addTracker(int64(len(byteMsgs)), func() { w.WriteHeader(http.StatusCreated) }, func() { w.WriteHeader(http.StatusBadRequest) })
		defer s.removeTracker(tracker.connID)

		for _, byteMsg = range byteMsgs {
			raw := s.rawpool.Get().(*model.RawTcpMessage)
			raw.Message = raw.Message[:len(byteMsg)]
			copy(raw.Message, byteMsg)
			raw.Decoder = config.DecoderBaseConfig
			raw.Client = r.RemoteAddr
			raw.ConnID = tracker.connID
			raw.ConfID = config.ConfID
			raw.LocalPort = int32(config.Port)
			s.rawMessagesQueue.Put(raw)
			base.IncomingMsgsCounter.WithLabelValues("httpserver", raw.Client, ports, "").Inc()
		}
		releaseBody(&bodyBuf)
		tracker.wait()
	}
}

func (s *HTTPServiceImpl) Write(p []byte) (int, error) {
	s.logger.Debug(string(bytes.TrimSpace(p)))
	return len(p), nil
}

func (s *HTTPServiceImpl) parse() error {
	gen := utils.NewGenerator()
	for {
		raw, err := s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			return nil
		}
		err = s.parseAndEnqueue(gen, raw)
		if err != nil {
			s.fail(raw.ConnID)
			base.ParsingErrorCounter.WithLabelValues("httpserver", raw.Client, raw.Decoder.Format).Inc()
			logg(s.logger, &raw.RawMessage).Warn(err.Error())
		} else {
			s.done(raw.ConnID)
		}
		s.rawpool.Put(raw)
		if err != nil && eerrors.IsFatal(err) {
			// stop processing when fatal error happens
			return err
		}
	}
}

func (s *HTTPServiceImpl) parseAndEnqueue(gen *utils.Generator, raw *model.RawTcpMessage) error {
	logger := s.logger.New(
		"protocol", "httpserver",
		"format", raw.Decoder.Format,
	)
	fulls, err := s.parseOne(raw)
	if err != nil {
		return eerrors.Wrap(err, "Error parsing HTTP server message")
	}
	for _, full := range fulls {
		defer model.FullFree(full)
		full.Uid = gen.Uid()

		err := s.reporter.Stash(full)

		if eerrors.IsFatal(err) {
			return eerrors.Wrap(err, "Fatal error stashing HTTP server message")
		}
		if err != nil {
			logger.Warn("Non-fatal error stashing HTTP message", "error", err)
		}
	}
	return nil
}

func (s *HTTPServiceImpl) parseOne(raw *model.RawTcpMessage) (fulls []*model.FullMessage, err error) {
	parser, err := s.parserEnv.GetParser(&raw.Decoder)
	if parser == nil || err != nil {
		return nil, decoders.DecodingError(eerrors.Wrapf(err, "Unknown decoder: %s", raw.Decoder.Format))
	}
	defer parser.Release()

	syslogMsgs, err := parser.Parse(raw.Message)
	if err != nil {
		return nil, decoders.DecodingError(eerrors.Wrap(err, "Parsing error"))
	}
	if len(syslogMsgs) == 0 {
		return nil, nil
	}
	fulls = make([]*model.FullMessage, 0, len(syslogMsgs))

	for _, syslogMsg := range syslogMsgs {
		if syslogMsg == nil {
			continue
		}
		full := model.FullFactoryFrom(syslogMsg)
		full.SourceType = "httpserver"
		full.SourcePort = raw.LocalPort
		full.ClientAddr = raw.Client
		full.ConfId = raw.ConfID
		full.ConnId = raw.ConnID
		fulls = append(fulls, full)
	}
	return fulls, nil
}

func (s *HTTPServiceImpl) Shutdown() {
	s.Stop()
}

func (s *HTTPServiceImpl) Stop() {
	s.stop()
	s.rawMessagesQueue.Dispose()
	s.wg.Wait()
}
