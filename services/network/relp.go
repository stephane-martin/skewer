package network

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/services/errors"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
	"github.com/stephane-martin/skewer/utils/queue/tcp"
)

var relpAnswersCounter *prometheus.CounterVec
var relpProtocolErrorsCounter *prometheus.CounterVec

func initRelpRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()

		relpAnswersCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_relp_answers_total",
				Help: "number of RSP answers sent back to the RELP client",
			},
			[]string{"status", "client"},
		)

		relpProtocolErrorsCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_relp_protocol_errors_total",
				Help: "Number of RELP protocol errors",
			},
			[]string{"client"},
		)

		base.Registry.MustRegister(
			relpAnswersCounter,
			relpProtocolErrorsCounter,
		)
	})
}

type RelpServerStatus int

const (
	Stopped RelpServerStatus = iota
	Started
	FinalStopped
	Waiting
)

type ackForwarder struct {
	succ sync.Map
	fail sync.Map
	comm sync.Map
	next uint32
}

func newAckForwarder() *ackForwarder {
	return &ackForwarder{}
}

func txnr2bytes(txnr int32) []byte {
	bs := make([]byte, 4)
	ux := uint32(txnr) << 1
	if txnr < 0 {
		ux = ^ux
	}
	binary.LittleEndian.PutUint32(bs, ux)
	return bs
}

func bytes2txnr(b []byte) int32 {
	ux := binary.LittleEndian.Uint32(b)
	x := int64(ux >> 1)
	if ux&1 != 0 {
		x = ^x
	}
	return int32(x)
}

func (f *ackForwarder) Received(connID utils.MyULID, txnr int32) {
	if c, ok := f.comm.Load(connID); ok {
		_ = c.(*queue.IntQueue).Put(txnr)
	}
}

func (f *ackForwarder) Commit(connID utils.MyULID) {
	if c, ok := f.comm.Load(connID); ok {
		_, _ = c.(*queue.IntQueue).Get()
	}
}

func (f *ackForwarder) NextToCommit(connID utils.MyULID) int32 {
	if c, ok := f.comm.Load(connID); ok {
		next, err := c.(*queue.IntQueue).Peek()
		if err != nil {
			return -1
		}
		return next
	}
	return -1
}

func (f *ackForwarder) ForwardSucc(connID utils.MyULID, txnr int32) {
	if q, ok := f.succ.Load(connID); ok {
		_ = q.(*queue.IntQueue).Put(txnr)
	}
}

func (f *ackForwarder) GetSucc(connID utils.MyULID) int32 {
	if q, ok := f.succ.Load(connID); ok {
		txnr, err := q.(*queue.IntQueue).Get()
		if err != nil {
			return -1
		}
		return txnr
	}
	return -1
}

func (f *ackForwarder) ForwardFail(connID utils.MyULID, txnr int32) {
	if q, ok := f.fail.Load(connID); ok {
		_ = q.(*queue.IntQueue).Put(txnr)
	}
}

func (f *ackForwarder) GetFail(connID utils.MyULID) int32 {
	if q, ok := f.fail.Load(connID); ok {
		txnr, err := q.(*queue.IntQueue).Get()
		if err != nil {
			return -1
		}
		return txnr
	}
	return -1
}

func (f *ackForwarder) AddConn() utils.MyULID {
	connID := utils.NewUid()
	f.succ.Store(connID, queue.NewIntQueue())
	f.fail.Store(connID, queue.NewIntQueue())
	f.comm.Store(connID, queue.NewIntQueue())
	return connID
}

func (f *ackForwarder) RemoveConn(connID utils.MyULID) {
	if q, ok := f.succ.Load(connID); ok {
		q.(*queue.IntQueue).Dispose()
		f.succ.Delete(connID)
	}
	if q, ok := f.fail.Load(connID); ok {
		q.(*queue.IntQueue).Dispose()
		f.fail.Delete(connID)
	}
	f.comm.Delete(connID)
}

func (f *ackForwarder) RemoveAll() {
	f.succ = sync.Map{}
	f.fail = sync.Map{}
	f.comm = sync.Map{}
}

func (f *ackForwarder) Wait(connID utils.MyULID) bool {
	qsucc, ok := f.succ.Load(connID)
	if !ok {
		return false
	}
	qfail, ok := f.fail.Load(connID)
	if !ok {
		return false
	}
	return queue.WaitOne(qsucc.(*queue.IntQueue), qfail.(*queue.IntQueue))
}

type meta struct {
	Txnr   int32
	ConnID utils.MyULID
}

type RelpService struct {
	impl           *RelpServiceImpl
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
	QueueSize      uint64
	logger         log15.Logger
	reporter       base.Reporter
	b              binder.Client
	sc             []conf.RELPSourceConfig
	pc             []conf.ParserConfig
	wg             sync.WaitGroup
	confined       bool
}

func NewRelpService(env *base.ProviderEnv) (base.Provider, error) {
	initRelpRegistry()
	s := RelpService{
		b:        env.Binder,
		logger:   env.Logger,
		reporter: env.Reporter,
		confined: env.Confined,
	}
	s.impl = NewRelpServiceImpl(env.Confined, env.Reporter, env.Binder, env.Logger)
	return &s, nil
}

func (s *RelpService) Type() base.Types {
	return base.RELP
}

func (s *RelpService) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *RelpService) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *RelpService) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *RelpService) Start() (infos []model.ListenerInfo, err error) {
	// the Relp service manages registration in Consul by itself and
	// therefore does not report infos
	//if capabilities.CapabilitiesSupported {
	//	s.logger.Debug("Capabilities", "caps", capabilities.GetCaps())
	//}
	infos = []model.ListenerInfo{}
	s.impl = NewRelpServiceImpl(s.confined, s.reporter, s.b, s.logger)
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			state := <-s.impl.StatusChan
			switch state {
			case FinalStopped:
				//s.impl.Logger.Debug("The RELP service has been definitely halted")
				//fmt.Fprintln(os.Stderr, "FINALSTOPPED")
				_ = s.reporter.Report([]model.ListenerInfo{})
				return

			case Stopped:
				//s.impl.Logger.Debug("The RELP service is stopped")
				s.impl.SetConf(s.sc, s.pc, s.QueueSize)
				infos, err := s.impl.Start()
				if err == nil {
					//fmt.Fprintln(os.Stderr, "STOPPED")
					err = s.reporter.Report(infos)
					if err != nil {
						s.impl.Logger.Error("Failed to report infos. Fatal error.", "error", err)
						s.dofatal()
					}
				} else {
					s.impl.Logger.Warn("The RELP service has failed to start", "error", err)
					//fmt.Fprintln(os.Stderr, "FAILSTART")
					err = s.reporter.Report([]model.ListenerInfo{})
					if err != nil {
						s.impl.Logger.Error("Failed to report infos. Fatal error.", "error", err)
						s.dofatal()
					} else {
						s.impl.StopAndWait()
					}
				}

			case Waiting:
				//s.impl.Logger.Debug("RELP waiting")
				go func() {
					time.Sleep(time.Duration(30) * time.Second)
					s.impl.EndWait()
				}()

			case Started:
				//s.impl.Logger.Debug("The RELP service has been started")
			}
		}
	}()

	s.impl.StatusChan <- Stopped // trigger the RELP service to start
	return
}

func (s *RelpService) Shutdown() {
	s.Stop()
}

func (s *RelpService) Stop() {
	s.impl.FinalStop()
	s.wg.Wait()
}

//func (s *RelpService) SetConf(sc []conf.RELPSourceConfig, pc []conf.ParserConfig, queueSize uint64) {
func (s *RelpService) SetConf(c conf.BaseConfig) {
	s.sc = c.RELPSource
	s.pc = c.Parsers
	s.QueueSize = c.Main.InputQueueSize
}

type RelpServiceImpl struct {
	StreamingService
	RelpConfigs      []conf.RELPSourceConfig
	status           RelpServerStatus
	StatusChan       chan RelpServerStatus
	reporter         base.Reporter
	rawMessagesQueue *tcp.Ring
	parsewg          sync.WaitGroup
	configs          map[utils.MyULID]conf.RELPSourceConfig
	forwarder        *ackForwarder
}

func NewRelpServiceImpl(confined bool, reporter base.Reporter, b binder.Client, logger log15.Logger) *RelpServiceImpl {
	s := RelpServiceImpl{
		status:    Stopped,
		reporter:  reporter,
		configs:   map[utils.MyULID]conf.RELPSourceConfig{},
		forwarder: newAckForwarder(),
	}
	s.StreamingService.init()
	s.StreamingService.BaseService.Logger = logger.New("class", "RelpServer")
	s.StreamingService.BaseService.Binder = b
	s.StreamingService.handler = RelpHandler{Server: &s}
	s.StreamingService.confined = confined
	s.StatusChan = make(chan RelpServerStatus, 10)
	return &s
}

func (s *RelpServiceImpl) Start() ([]model.ListenerInfo, error) {
	s.LockStatus()
	defer s.UnlockStatus()
	if s.status == FinalStopped {
		return nil, errors.ServerDefinitelyStopped
	}
	if s.status != Stopped && s.status != Waiting {
		return nil, errors.ServerNotStopped
	}

	infos := s.initTCPListeners()
	if len(infos) == 0 {
		s.Logger.Info("RELP service not started: no listener")
		return infos, nil
	}

	s.Logger.Info("Listening on RELP", "nb_services", len(infos))

	s.rawMessagesQueue = tcp.NewRing(s.QueueSize)
	s.configs = map[utils.MyULID]conf.RELPSourceConfig{}

	for _, l := range s.UnixListeners {
		s.configs[l.Conf.ConfID] = conf.RELPSourceConfig(l.Conf)
	}
	for _, l := range s.TcpListeners {
		s.configs[l.Conf.ConfID] = conf.RELPSourceConfig(l.Conf)
	}

	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.parsewg.Add(1)
		go s.Parse()
	}

	s.status = Started
	s.StatusChan <- Started

	s.Listen()
	return infos, nil
}

func (s *RelpServiceImpl) Stop() {
	s.LockStatus()
	s.doStop(false, false)
	s.UnlockStatus()
}

func (s *RelpServiceImpl) FinalStop() {
	s.LockStatus()
	s.doStop(true, false)
	s.UnlockStatus()
}

func (s *RelpServiceImpl) StopAndWait() {
	s.LockStatus()
	s.doStop(false, true)
	s.UnlockStatus()
}

func (s *RelpServiceImpl) EndWait() {
	s.LockStatus()
	if s.status != Waiting {
		s.UnlockStatus()
		return
	}
	s.status = Stopped
	s.StatusChan <- Stopped
	s.UnlockStatus()
}

func (s *RelpServiceImpl) doStop(final bool, wait bool) {
	if final && (s.status == Waiting || s.status == Stopped || s.status == FinalStopped) {
		if s.status != FinalStopped {
			s.status = FinalStopped
			s.StatusChan <- FinalStopped
			close(s.StatusChan)
		}
		return
	}

	if s.status == Stopped || s.status == FinalStopped || s.status == Waiting {
		if s.status == Stopped && wait {
			s.status = Waiting
			s.StatusChan <- Waiting
		}
		return
	}

	s.resetTCPListeners() // makes the listeners stop
	// no more message will arrive in rawMessagesQueue
	if s.rawMessagesQueue != nil {
		s.rawMessagesQueue.Dispose()
	}
	// the parsers consume the rest of rawMessagesQueue, then they stop
	s.parsewg.Wait() // wait that the parsers have stopped

	// after the parsers have stopped, we can close the queues
	s.forwarder.RemoveAll()
	// wait that all goroutines have ended
	s.wg.Wait()

	if final {
		s.status = FinalStopped
		s.StatusChan <- FinalStopped
		close(s.StatusChan)
	} else if wait {
		s.status = Waiting
		s.StatusChan <- Waiting
	} else {
		s.status = Stopped
		s.StatusChan <- Stopped
	}
}

func (s *RelpServiceImpl) SetConf(sc []conf.RELPSourceConfig, pc []conf.ParserConfig, queueSize uint64) {
	tcpConfigs := []conf.TCPSourceConfig{}
	for _, c := range sc {
		tcpConfigs = append(tcpConfigs, conf.TCPSourceConfig(c))
	}
	s.StreamingService.SetConf(tcpConfigs, pc, queueSize, 132000)
	s.BaseService.Pool = &sync.Pool{New: func() interface{} {
		return &model.RawTcpMessage{Message: make([]byte, 132000)}
	}}
}

func (s *RelpServiceImpl) parseOne(raw *model.RawTcpMessage, e *base.ParsersEnv, gen *utils.Generator) {

	logger := s.Logger.New(
		"protocol", "relp",
		"client", raw.Client,
		"local_port", raw.LocalPort,
		"unix_socket_path", raw.UnixSocketPath,
		"format", raw.Format,
		"txnr", raw.Txnr,
	)
	parser, err := e.GetParser(raw.Format)
	if parser == nil || err != nil {
		s.forwarder.ForwardFail(raw.ConnID, raw.Txnr)
		logger.Crit("Unknown parser")
		return
	}
	decoder := utils.SelectDecoder(raw.Encoding)
	syslogMsg, err := parser(raw.Message, decoder)
	if err != nil {
		//logger.Warn("Parsing error", "message", string(raw.Message[:raw.Size]), "error", err)
		logger.Warn("Parsing error", "error", err)
		s.forwarder.ForwardFail(raw.ConnID, raw.Txnr)
		base.ParsingErrorCounter.WithLabelValues("relp", raw.Client, raw.Format).Inc()
		return
	}
	if syslogMsg == nil {
		s.forwarder.ForwardSucc(raw.ConnID, raw.Txnr)
		return
	}
	if raw.Client != "" {
		syslogMsg.SetProperty("skewer", "client", raw.Client)
	}
	if raw.LocalPort != 0 {
		syslogMsg.SetProperty("skewer", "localport", strconv.FormatInt(int64(raw.LocalPort), 10))
	}
	if raw.UnixSocketPath != "" {
		syslogMsg.SetProperty("skewer", "socketpath", raw.UnixSocketPath)
	}

	full := model.FullFactoryFrom(syslogMsg)
	full.Txnr = raw.Txnr
	full.ConfId = raw.ConfID
	full.ConnId = raw.ConnID
	full.Uid = gen.Uid()
	defer model.FullFree(full)
	f, nonf := s.reporter.Stash(full)
	if f == nil && nonf == nil {
		s.forwarder.ForwardSucc(full.ConnId, full.Txnr)
	} else if f != nil {
		s.forwarder.ForwardFail(full.ConnId, full.Txnr)
		logger.Error("Fatal error pushing RELP message to the Store", "err", f)
		s.StopAndWait()
		return
	} else {
		s.forwarder.ForwardFail(full.ConnId, full.Txnr)
		logger.Warn("Non fatal error pushing RELP message to the Store", "err", nonf)
	}
}

func (s *RelpServiceImpl) Parse() {
	defer s.parsewg.Done()

	e := base.NewParsersEnv(s.ParserConfigs, s.Logger)

	var raw *model.RawTcpMessage
	var err error
	gen := utils.NewGenerator()

	for {
		raw, err = s.rawMessagesQueue.Get()
		if err != nil {
			return
		}
		if raw == nil {
			s.Logger.Error("rawMessagesQueue returns nil, should not happen!")
			return
		}
		s.parseOne(raw, e, gen)
		s.Pool.Put(raw)
	}

}

func (s *RelpServiceImpl) handleResponses(conn net.Conn, connID utils.MyULID, client string, logger log15.Logger) {
	defer func() {
		s.wg.Done()
	}()

	successes := map[int32]bool{}
	failures := map[int32]bool{}
	var err error

	writeSuccess := func(txnr int32) (err error) {
		_, err = fmt.Fprintf(conn, "%d rsp 6 200 OK\n", txnr)
		return err
	}

	writeFailure := func(txnr int32) (err error) {
		_, err = fmt.Fprintf(conn, "%d rsp 6 500 KO\n", txnr)
		return err
	}

	for s.forwarder.Wait(connID) {
		currentTxnr := s.forwarder.GetSucc(connID)
		if currentTxnr != -1 {
			//logger.Debug("New success to report to client", "txnr", currentTxnr)
			successes[currentTxnr] = true
		}

		currentTxnr = s.forwarder.GetFail(connID)
		if currentTxnr != -1 {
			//logger.Debug("New failure to report to client", "txnr", currentTxnr)
			failures[currentTxnr] = true
		}

		// rsyslog expects the ACK/txnr correctly and monotonously ordered
		// so we need a bit of cooking to ensure that
	Cooking:
		for {
			next := s.forwarder.NextToCommit(connID)
			if next == -1 {
				break Cooking
			}
			//logger.Debug("Next to commit", "connid", connID, "txnr", next)
			if successes[next] {
				err = writeSuccess(next)
				if err == nil {
					//logger.Debug("ACK to client", "connid", connID, "tnxr", next)
					delete(successes, next)
					relpAnswersCounter.WithLabelValues("200", client).Inc()
				}
			} else if failures[next] {
				err = writeFailure(next)
				if err == nil {
					//logger.Debug("NACK to client", "connid", connID, "txnr", next)
					delete(failures, next)
					relpAnswersCounter.WithLabelValues("500", client).Inc()
				}
			} else {
				break Cooking
			}

			if err == nil {
				s.forwarder.Commit(connID)
			} else if err == io.EOF {
				// client is gone
				return
			} else if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				logger.Info("Timeout error writing RELP response to client", "error", err)
			} else {
				logger.Warn("Unexpected error writing RELP response to client", "error", err)
				return
			}
		}
	}
}

type RelpHandler struct {
	Server *RelpServiceImpl
}

func (h RelpHandler) HandleConnection(conn net.Conn, c conf.TCPSourceConfig) {
	// http://www.rsyslog.com/doc/relp.html
	config := conf.RELPSourceConfig(c)
	s := h.Server
	s.AddConnection(conn)
	connID := s.forwarder.AddConn()
	scanner := bufio.NewScanner(conn)
	logger := s.Logger.New("ConnID", connID)

	defer func() {
		logger.Info("Scanning the RELP stream has ended", "error", scanner.Err())
		s.forwarder.RemoveConn(connID)
		s.RemoveConnection(conn)
		s.wg.Done()
	}()

	var relpIsOpen bool

	client := ""
	path := ""
	remote := conn.RemoteAddr()

	var localPort int32
	if remote == nil {
		client = "localhost"
		localPort = 0
		path = conn.LocalAddr().String()
	} else {
		client = strings.Split(remote.String(), ":")[0]
		local := conn.LocalAddr()
		if local != nil {
			s := strings.Split(local.String(), ":")
			localPort, _ = utils.Atoi32(s[len(s)-1])
		}
	}
	client = strings.TrimSpace(client)
	path = strings.TrimSpace(path)
	localPortStr := strconv.FormatInt(int64(localPort), 10)

	logger = logger.New(
		"protocol", "relp",
		"client", client,
		"local_port", localPort,
		"unix_socket_path", path,
		"format", config.Format,
	)
	logger.Info("New client connection")
	base.ClientConnectionCounter.WithLabelValues("relp", client, localPortStr, path).Inc()

	s.wg.Add(1)
	go s.handleResponses(conn, connID, client, logger)

	timeout := config.Timeout
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}
	scanner.Split(utils.RelpSplit)
	scanner.Buffer(make([]byte, 0, 132000), 132000)
	var rawmsg *model.RawTcpMessage
	var previous = int32(-1)

Loop:
	for scanner.Scan() {
		splits := bytes.SplitN(scanner.Bytes(), sp, 4)
		txnr, _ := utils.Atoi32(string(splits[0]))
		if txnr <= previous {
			logger.Warn("TXNR did not increase", "previous", previous, "current", txnr)
			relpProtocolErrorsCounter.WithLabelValues(client).Inc()
			return
		}
		previous = txnr
		command := string(splits[1])
		datalen, _ := strconv.Atoi(string(splits[2]))
		data := []byte{}
		if datalen != 0 {
			if len(splits) == 4 {
				data = bytes.Trim(splits[3], " \r\n")
			} else {
				logger.Warn("datalen is non-null, but no data is provided", "datalen", datalen)
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
		}
		switch command {
		case "open":
			if relpIsOpen {
				logger.Warn("Received open command twice")
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			fmt.Fprintf(conn, "%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, string(data))
			relpIsOpen = true
			logger.Info("Received 'open' command")
		case "close":
			if !relpIsOpen {
				logger.Warn("Received close command before open")
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			fmt.Fprintf(conn, "%d rsp 0\n0 serverclose 0\n", txnr)
			logger.Info("Received 'close' command")
			return
		case "syslog":
			if !relpIsOpen {
				logger.Warn("Received syslog command before open")
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			s.forwarder.Received(connID, txnr)
			if len(data) == 0 {
				s.forwarder.ForwardSucc(connID, txnr)
				continue Loop
			}
			if s.MaxMessageSize > 0 && len(data) > s.MaxMessageSize {
				logger.Warn("Message too large", "max", s.MaxMessageSize, "length", len(buf))
				relpProtocolErrorsCounter.WithLabelValues(client).Inc()
				return
			}
			rawmsg = s.Pool.Get().(*model.RawTcpMessage)
			rawmsg.Txnr = txnr
			rawmsg.Client = client
			rawmsg.LocalPort = localPort
			rawmsg.UnixSocketPath = path
			rawmsg.ConfID = config.ConfID
			rawmsg.Encoding = config.Encoding
			rawmsg.Format = config.Format
			rawmsg.ConnID = connID
			rawmsg.Message = rawmsg.Message[:len(data)]
			copy(rawmsg.Message, data)
			err := s.rawMessagesQueue.Put(rawmsg)
			if err != nil {
				s.Logger.Error("Failed to enqueue new raw RELP message", "error", err)
				return
			}
			base.IncomingMsgsCounter.WithLabelValues("relp", client, localPortStr, path).Inc()
			//logger.Debug("RELP client received a syslog message")
		default:
			logger.Warn("Unknown RELP command", "command", command)
			relpProtocolErrorsCounter.WithLabelValues(client).Inc()
			return
		}
		if timeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(timeout))
		}

	}
}
