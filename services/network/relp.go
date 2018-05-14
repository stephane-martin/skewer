package network

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/looplab/fsm"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue/intq"
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
		_ = c.(*intq.Ring).Put(txnr)
	}
}

func (f *ackForwarder) NextToCommit(connID utils.MyULID) int32 {
	if c, ok := f.comm.Load(connID); ok {
		next, err := c.(*intq.Ring).Poll(time.Nanosecond)
		if err != nil {
			return -1
		}
		return next
	}
	return -1
}

func (f *ackForwarder) ForwardSucc(connID utils.MyULID, txnr int32) {
	if q, ok := f.succ.Load(connID); ok {
		_ = q.(*intq.Ring).Put(txnr)
	}
}

func (f *ackForwarder) ForwardFail(connID utils.MyULID, txnr int32) {
	if q, ok := f.fail.Load(connID); ok {
		_ = q.(*intq.Ring).Put(txnr)
	}
}

func (f *ackForwarder) GetSuccAndFail(connID utils.MyULID) (success int32, failure int32) {
	var w utils.ExpWait
	var err error
	success = -1
	failure = -1

	if q1, ok := f.succ.Load(connID); ok {
		if q2, ok := f.fail.Load(connID); ok {
			qsucc := q1.(*intq.Ring)
			qfail := q2.(*intq.Ring)
			for {
				if qsucc.IsDisposed() || qfail.IsDisposed() {
					return -1, -1
				}
				if qsucc.Len() == 0 && qfail.Len() == 0 {
					w.Wait()
					continue
				}
				if qsucc.Len() > 0 {
					success, err = qsucc.Get()
					if err == eerrors.ErrQDisposed {
						return -1, -1
					}
					if err == eerrors.ErrQTimeout {
						success = -1
					}
				}
				if qfail.Len() > 0 {
					failure, err = qfail.Get()
					if err == eerrors.ErrQDisposed {
						return -1, -1
					}
					if err == eerrors.ErrQTimeout {
						failure = -1
					}
				}
				if success == -1 && failure == -1 {
					w.Wait()
					continue
				}
				return success, failure
			}
		}
	}
	return -1, -1
}

func (f *ackForwarder) AddConn(qsize uint64) utils.MyULID {
	connID := utils.NewUid()
	f.succ.Store(connID, intq.NewRing(qsize))
	f.fail.Store(connID, intq.NewRing(qsize))
	f.comm.Store(connID, intq.NewRing(qsize))
	return connID
}

func (f *ackForwarder) RemoveConn(connID utils.MyULID) {
	if q, ok := f.succ.Load(connID); ok {
		q.(*intq.Ring).Dispose()
		f.succ.Delete(connID)
	}
	if q, ok := f.fail.Load(connID); ok {
		q.(*intq.Ring).Dispose()
		f.fail.Delete(connID)
	}
	f.comm.Delete(connID)
}

func (f *ackForwarder) RemoveAll() {
	f.succ = sync.Map{}
	f.fail = sync.Map{}
	f.comm = sync.Map{}
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
	RelpConfigs []conf.RELPSourceConfig
	status      RelpServerStatus
	StatusChan  chan RelpServerStatus
	reporter    base.Reporter
	rawQ        *tcp.Ring
	parsewg     sync.WaitGroup
	configs     map[utils.MyULID]conf.RELPSourceConfig
	forwarder   *ackForwarder
	parserEnv   *decoders.ParsersEnv
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
		return nil, ServerDefinitelyStopped
	}
	if s.status != Stopped && s.status != Waiting {
		return nil, ServerNotStopped
	}

	infos := s.initTCPListeners()
	if len(infos) == 0 {
		s.Logger.Info("RELP service not started: no listener")
		return infos, nil
	}

	s.Logger.Info("Listening on RELP", "nb_services", len(infos))

	s.rawQ = tcp.NewRing(s.QueueSize)
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
		go func() {
			// Parse() returns an error if something fatal happened
			// in that case we stop the RELP service (and try to restart it after)
			err := s.Parse()
			s.parsewg.Done()
			if err != nil {
				s.Logger.Error(err.Error())
				go s.StopAndWait()
			}
		}()
	}

	s.status = Started
	s.StatusChan <- Started

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.Listen()
	}()
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
	s.CloseConnections()
	// no more message will arrive in rawMessagesQueue
	if s.rawQ != nil {
		s.rawQ.Dispose()
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
	// MaxMessageSize is 132000 bytes
	s.StreamingService.SetConf(tcpConfigs, pc, queueSize, 132000)
	s.parserEnv = decoders.NewParsersEnv(s.ParserConfigs, s.Logger)
}

func (s *RelpServiceImpl) parseOne(raw *model.RawTcpMessage, gen *utils.Generator) error {
	parser, err := s.parserEnv.GetParser(&raw.Decoder)
	if parser == nil || err != nil {
		return decoders.DecodingError(eerrors.Wrapf(err, "Unknown decoder: %s", raw.Decoder.Format))
	}
	defer parser.Release()

	syslogMsgs, err := parser.Parse(raw.Message)
	if err != nil {
		return decoders.DecodingError(eerrors.Wrap(err, "Parsing error"))
	}

	var syslogMsg *model.SyslogMessage
	var full *model.FullMessage

	for _, syslogMsg = range syslogMsgs {
		if syslogMsg == nil {
			continue
		}

		full = model.FullFactoryFrom(syslogMsg)
		full.Txnr = raw.Txnr
		full.ConfId = raw.ConfID
		full.Uid = gen.Uid()
		full.SourceType = "relp"
		full.ClientAddr = raw.Client
		full.SourcePort = raw.LocalPort
		full.SourcePath = raw.UnixSocketPath

		err := s.reporter.Stash(full)
		model.FullFree(full)
		if err != nil {
			// a non fatal error is typically an error marshalling the message to the communication pipe with the coordinator
			// such an error is not supposed to happen. if it does, we just log and continue the processing of remaining syslogMsgs
			logg(s.Logger, &raw.RawMessage).Warn("Error stashing RELP message", "error", err)
			if eerrors.IsFatal(err) {
				return eerrors.Wrap(err, "Fatal error pushing RELP message to the Store")
			}
		}
	}
	return nil
}

func (s *RelpServiceImpl) Parse() error {
	gen := utils.NewGenerator()

	for {
		raw, err := s.rawQ.Get()
		if err != nil || raw == nil {
			return nil
		}

		err = s.parseOne(raw, gen)
		if err != nil {
			s.forwarder.ForwardFail(raw.ConnID, raw.Txnr)
			base.ParsingErrorCounter.WithLabelValues("relp", raw.Client, raw.Decoder.Format).Inc()
			logg(s.Logger, &raw.RawMessage).Warn(err.Error())
		} else {
			s.forwarder.ForwardSucc(raw.ConnID, raw.Txnr)
		}

		model.RawTCPFree(raw)

		if err != nil && eerrors.IsFatal(err) {
			// stop processing when fatal error happens
			return err
		}
	}
}

func writeSuccess(conn net.Conn, txnr int32) (err error) {
	_, err = fmt.Fprintf(conn, "%d rsp 6 200 OK\n", txnr)
	return err
}

func writeFailure(conn net.Conn, txnr int32) (err error) {
	_, err = fmt.Fprintf(conn, "%d rsp 6 500 KO\n", txnr)
	return err
}

func (s *RelpServiceImpl) handleResponses(conn net.Conn, connID utils.MyULID, client string, logger log15.Logger) error {
	successes := map[int32]bool{}
	failures := map[int32]bool{}
	var err error
	var ok1, ok2 bool

	var next int32 = -1

	for {
		txnrSuccess, txnrFailure := s.forwarder.GetSuccAndFail(connID)

		if txnrSuccess == -1 && txnrFailure == -1 {
			return io.EOF
		}

		if txnrSuccess != -1 {
			//logger.Debug("New success to report to client", "txnr", currentTxnr)
			_, ok1 = successes[txnrSuccess]
			_, ok2 = failures[txnrSuccess]
			if !ok1 && !ok2 {
				successes[txnrSuccess] = true
			}
		}

		if txnrFailure != -1 {
			//logger.Debug("New failure to report to client", "txnr", currentTxnr)
			_, ok1 = successes[txnrFailure]
			_, ok2 = failures[txnrFailure]
			if !ok1 && !ok2 {
				failures[txnrFailure] = true
			}
		}

		// rsyslog expects the ACK/txnr correctly and monotonicly ordered
		// so we need a bit of cooking to ensure that
	Cooking:
		for {
			if next == -1 {
				next = s.forwarder.NextToCommit(connID)
			}
			if next == -1 {
				break Cooking
			}
			//logger.Debug("Next to commit", "connid", connID, "txnr", next)
			if successes[next] {
				err = writeSuccess(conn, next)
				if err == nil {
					successes[next] = false
					relpAnswersCounter.WithLabelValues("200", client).Inc()
				}
			} else if failures[next] {
				err = writeFailure(conn, next)
				if err == nil {
					failures[next] = false
					relpAnswersCounter.WithLabelValues("500", client).Inc()
				}
			} else {
				break Cooking
			}

			if err == nil {
				next = -1
			} else if eerrors.HasFileClosed(err) {
				return io.EOF // client is gone
			} else if eerrors.IsTimeout(err) {
				logger.Warn("Timeout error writing RELP response to client", "error", err)
			} else {
				return eerrors.Wrap(err, "Unexpected error writing RELP response to client")
			}
		}
	}
}

type RelpHandler struct {
	Server *RelpServiceImpl
}

func (h RelpHandler) HandleConnection(conn net.Conn, c conf.TCPSourceConfig) (err error) {
	// http://www.rsyslog.com/doc/relp.html
	config := conf.RELPSourceConfig(c)
	s := h.Server
	s.AddConnection(conn)
	connID := s.forwarder.AddConn(s.QueueSize)
	props := eprops(conn)
	l := makeLogger(s.Logger, props, "relp")
	l.Info("New client")
	defer l.Debug("Client gone away")
	clientCounter(props, "relp")

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		e := s.handleResponses(conn, connID, props.Client, l)
		if e != nil && !eerrors.HasFileClosed(e) {
			s.Logger.Warn("Unexpected error in RELP handleResponses", "error", e, "connID", connID.String())
		}
	}()

	wg.Add(1)
	go func() {
		defer func() {
			s.forwarder.RemoveConn(connID) // this makes handleResponses return
			s.RemoveConnection(conn)
			wg.Done()
		}()
		e := scan(l, s.forwarder, s.rawQ, conn, config.Timeout, config.ConfID, connID, s.MaxMessageSize, config.DecoderBaseConfig, props)
		if e != nil && !eerrors.HasFileClosed(e) {
			err = eerrors.Wrap(e, "RELP scanning error")
		}
	}()

	wg.Wait()
	return err
}

func scan(l log15.Logger, f *ackForwarder, rawq *tcp.Ring, c net.Conn, tout time.Duration, cfid, cnid utils.MyULID, msiz int, dc conf.DecoderBaseConfig, props tcpProps) (err error) {
	var previous = int32(-1)
	var command string
	var txnr int32
	var splits [][]byte
	var data []byte

	machine := newMachine(l, f, rawq, c, cfid, cnid, msiz, dc, props)

	if tout > 0 {
		_ = c.SetReadDeadline(time.Now().Add(tout))
	}
	scanner := bufio.NewScanner(c)
	scanner.Split(utils.RelpSplit)
	scanner.Buffer(make([]byte, 0, 132000), 132000)

	defer func() {
		if e := eerrors.Err(recover()); e != nil {
			err = eerrors.Wrap(e, "Scanner panicked in RELP service")
		}
	}()

	for scanner.Scan() {
		splits = bytes.SplitN(scanner.Bytes(), sp, 3)
		txnr, err = utils.Atoi32(string(splits[0]))
		if err != nil {
			relpProtocolErrorsCounter.WithLabelValues(props.Client).Inc()
			return eerrors.Wrap(err, "Badly formed TXNR")
		}
		if txnr <= previous {
			relpProtocolErrorsCounter.WithLabelValues(props.Client).Inc()
			return eerrors.Errorf("TXNR has not increased (previous = %d, current = %d)", previous, txnr)
		}
		previous = txnr
		command = string(splits[1])
		data = data[:0]
		if len(splits) == 3 {
			data = bytes.TrimSpace(splits[2])
		}

		err = machine.Event(command, txnr, data)
		if err != nil {
			switch err.(type) {
			case fsm.UnknownEventError:
				relpProtocolErrorsCounter.WithLabelValues(props.Client).Inc()
				return eerrors.Wrapf(err, "Unknown RELP command: %s", command)
			case fsm.InvalidEventError:
				relpProtocolErrorsCounter.WithLabelValues(props.Client).Inc()
				return eerrors.Wrapf(err, "Invalid RELP command: %s", command)
			case fsm.InternalError:
				relpProtocolErrorsCounter.WithLabelValues(props.Client).Inc()
				return eerrors.Wrap(err, "Internal RELP state machine error")
			case fsm.NoTransitionError:
				// syslog does not change opened/closed state
				// nothing to do
			default:
				if eerrors.HasFileClosed(err) {
					return io.EOF
				}
				return err
			}
		}
		if tout > 0 {
			_ = c.SetReadDeadline(time.Now().Add(tout))
		}
	}
	err = scanner.Err()
	if eerrors.HasFileClosed(err) {
		return io.EOF
	}
	return err
}

func newMachine(l log15.Logger, fwder *ackForwarder, rawq *tcp.Ring, conn io.Writer, confID, connID utils.MyULID, msiz int, dc conf.DecoderBaseConfig, props tcpProps) *fsm.FSM {
	factory := makeRawTCPFactory(props, confID, dc)
	// TODO: PERF: fsm protects internal variables (states, events) with mutexes. We don't really need the mutexes here.
	return fsm.NewFSM(
		"closed",
		fsm.Events{
			fsm.EventDesc{Name: "open", Src: []string{"closed"}, Dst: "opened"},
			fsm.EventDesc{Name: "close", Src: []string{"opened"}, Dst: "closed"},
			fsm.EventDesc{Name: "syslog", Src: []string{"opened"}, Dst: "opened"},
		},
		fsm.Callbacks{
			"after_syslog": func(e *fsm.Event) {
				txnr := e.Args[0].(int32)
				data := e.Args[1].([]byte)
				fwder.Received(connID, txnr)
				if len(data) == 0 {
					fwder.ForwardSucc(connID, txnr)
					return
				}
				if msiz > 0 && len(data) > msiz {
					relpProtocolErrorsCounter.WithLabelValues(props.Client).Inc()
					e.Err = fmt.Errorf("Message too large: %d > %d", len(data), msiz)
					return
				}
				rawmsg := factory(data)
				rawmsg.Txnr = txnr
				rawmsg.ConnID = connID
				err := rawq.Put(rawmsg)
				if err != nil {
					e.Err = eerrors.Fatal(eerrors.Wrap(err, "Failed to enqueue new raw RELP message"))
					return
				}
				incomingCounter(props, "relp")
			},
			"enter_closed": func(e *fsm.Event) {
				txnr := e.Args[0].(int32)
				fmt.Fprintf(conn, "%d rsp 0\n0 serverclose 0\n", txnr)
				l.Debug("Received 'close' command")
				e.Err = io.EOF
			},
			"enter_opened": func(e *fsm.Event) {
				txnr := e.Args[0].(int32)
				data := e.Args[1].([]byte)
				fmt.Fprintf(conn, "%d rsp %d 200 OK\n%s\n", txnr, len(data)+7, string(data))
				l.Debug("Received 'open' command")
			},
		},
	)
}
