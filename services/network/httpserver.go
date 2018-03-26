package network

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue/tcp"
	"github.com/valyala/bytebufferpool"
)

var requestBodyPool bytebufferpool.Pool

var copyBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
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
		count:        count,
		connID:       utils.NewUid(),
		callbackOK:   callbackOK,
		callbackFail: callbackFail,
	}
	r.Lock()
	return &r
}

type requestTracker struct {
	count        int64
	callbackOK   func()
	callbackFail func()
	connID       utils.MyULID
	failed       int32
	sync.Mutex
	sync.Once
}

func (r *requestTracker) done() {
	if atomic.AddInt64(&r.count, -1) == 0 && atomic.LoadInt32(&r.failed) == 0 {
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
	atomic.AddInt64(&r.count, -1)
	if atomic.CompareAndSwapInt32(&r.failed, 0, 1) {
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
	stopChan         chan struct{}
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
		stopChan: make(chan struct{}),
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
	s.stopChan = make(chan struct{})
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}
	for _, config := range s.configs {
		s.wg.Add(1)
		go s.startOne(config)
	}
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.wg.Add(1)
		go s.parse()
	}

	return infos, nil
}

func (s *HTTPServiceImpl) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *HTTPServiceImpl) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *HTTPServiceImpl) startOne(config conf.HTTPServerSourceConfig) error {
	defer s.wg.Done()
	var err error
	var listener net.Listener

	hostport := net.JoinHostPort(config.BindAddr, strconv.FormatInt(int64(config.Port), 10))
	if config.DisableConnKeepAlive {
		listener, err = s.binder.Listen("tcp", hostport)
	} else {
		listener, err = s.binder.ListenKeepAlive("tcp", hostport, config.ConnKeepAlivePeriod)
	}
	if err != nil {
		return err
	}
	defer listener.Close()

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
	if config.TLSEnabled {
		tlsConf, err := utils.NewTLSConfig("", config.CAFile, config.CAPath, config.CertFile, config.KeyFile, false, s.confined)
		if err != nil {
			return err
		}
		tlsConf.ClientAuth = config.GetClientAuthType()
		server.TLSConfig = tlsConf
		return server.ServeTLS(listener, "", "")
	}
	s.wg.Add(1)
	go func() {
		<-s.stopChan
		server.Close()
		s.wg.Done()
	}()
	return server.Serve(listener)
}

func (s *HTTPServiceImpl) handler(config conf.HTTPServerSourceConfig) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// get the request body into bodyBuf
		if config.MaxBodySize > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, config.MaxBodySize)
		}
		bodyBuf := requestBodyPool.Get()
		defer requestBodyPool.Put(bodyBuf)
		bodyBuf.Reset()
		_, err := copyZeroAlloc(bodyBuf, r.Body)
		r.Body.Close()
		if err != nil {
			s.logger.Warn("Error reading request body", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Method != "POST" {
			s.logger.Warn("Request method is not POST", "method", r.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// parse body to get syslog messages
		if config.DisableMultiple {
			// the body should contain only one message
			raw := s.rawpool.Get().(*model.RawTcpMessage)
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
			raw.Message = raw.Message[:len(tmp)]
			copy(raw.Message, tmp)
			raw.Decoder = config.DecoderBaseConfig
			raw.Client = r.RemoteAddr
			raw.ConfID = config.ConfID
			raw.LocalPort = int32(config.Port)

			fulls, err := s.parseOne(raw)
			defer s.rawpool.Put(raw)
			if err != nil {
				s.logger.Warn("Error parsing message", "error", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			var full *model.FullMessage
			for _, full = range fulls {
				defer model.FullFree(full)
				full.Uid = utils.NewUid()

				fatal, nonfatal := s.reporter.Stash(full)
				if fatal != nil {
					w.WriteHeader(http.StatusInternalServerError)
					s.logger.Error("Fatal error stashing HTTP message", "error", fatal)
					close(s.fatalErrorChan)
					return
				} else if nonfatal != nil {
					s.logger.Warn("Non-fatal error stashing HTTP message", "error", nonfatal)
				}
			}
			//base.IncomingMsgsCounter.WithLabelValues("kafka", raw.Brokers, "", "").Inc()
			w.WriteHeader(http.StatusCreated)

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
		s.logger.Debug("Multiple messages received by HTTP", "nb_messages", len(byteMsgs))
		tracker := s.addTracker(int64(len(byteMsgs)), func() { w.WriteHeader(http.StatusCreated) }, func() { w.WriteHeader(http.StatusBadRequest) })
		defer s.removeTracker(tracker.connID)

		for _, byteMsg = range byteMsgs {
			raw := s.rawpool.Get().(*model.RawTcpMessage)
			raw.Message = raw.Message[:len(byteMsg)]
			// we copy the byteMsg so that we can release the bodyBuf afterwards
			copy(raw.Message, byteMsg)
			raw.Decoder = config.DecoderBaseConfig
			raw.Client = r.RemoteAddr
			raw.ConnID = tracker.connID
			raw.ConfID = config.ConfID
			raw.LocalPort = int32(config.Port)
			s.rawMessagesQueue.Put(raw)
		}
		tracker.wait()
	}
}

func (s *HTTPServiceImpl) Write(p []byte) (int, error) {
	s.logger.Debug(string(bytes.TrimSpace(p)))
	return len(p), nil
}

func (s *HTTPServiceImpl) parse() {
	defer s.wg.Done()
	gen := utils.NewGenerator()
	for {
		raw, err := s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			return
		}
		err = s.parseAndEnqueue(gen, raw)
		s.rawpool.Put(raw)
		if err != nil {
			return
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
		logger.Warn("Error parsing message", "error", err)
		s.fail(raw.ConnID)
		return nil
	}
	var full *model.FullMessage
	for _, full = range fulls {
		defer model.FullFree(full)
		full.Uid = gen.Uid()

		fatal, nonfatal := s.reporter.Stash(full)

		if fatal != nil {
			logger.Error("Fatal error stashing HTTP message", "error", fatal)
			s.fail(raw.ConnID)
			s.dofatal()
			return fatal
		} else if nonfatal != nil {
			logger.Warn("Non-fatal error stashing HTTP message", "error", nonfatal)
			// TODO/ think about non-fatal handling
		}
	}
	s.done(raw.ConnID)
	return nil
	//base.IncomingMsgsCounter.WithLabelValues("kafka", raw.Brokers, "", "").Inc()
}

func (s *HTTPServiceImpl) parseOne(raw *model.RawTcpMessage) (fulls []*model.FullMessage, err error) {
	parser, err := s.parserEnv.GetParser(&raw.Decoder)
	if parser == nil || err != nil {
		return nil, fmt.Errorf("Unknown parser")
	}
	defer parser.Release()

	syslogMsgs, err := parser.Parse(raw.Message)
	if err != nil {
		return nil, err
		//base.ParsingErrorCounter.WithLabelValues("kafka", raw.Brokers, raw.Format).Inc()
		//logger.Info("Parsing error", "message", string(raw.Message), "error", err)
	}
	if len(syslogMsgs) == 0 {
		return nil, nil
	}
	fulls = make([]*model.FullMessage, 0, len(syslogMsgs))
	var syslogMsg *model.SyslogMessage
	var full *model.FullMessage

	for _, syslogMsg = range syslogMsgs {
		if syslogMsg == nil {
			continue
		}
		full = model.FullFactoryFrom(syslogMsg)
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
	close(s.stopChan)
	s.rawMessagesQueue.Dispose()
	s.wg.Wait()
}
