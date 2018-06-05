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
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue/intq"
	"github.com/stephane-martin/skewer/utils/queue/tcp"
	"github.com/stephane-martin/skewer/utils/waiter"
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

func countRelpProtocolError(client string) {
	relpProtocolErrorsCounter.WithLabelValues(client).Inc()
}

func countRelpAnswer(client string, status int) {
	relpAnswersCounter.WithLabelValues(
		strconv.FormatInt(int64(status), 10),
		client,
	).Inc()
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
	w := waiter.Default()
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
	StreamingService
	fatalErrorChan chan struct{}
	fatalOnce      sync.Once
	ACKQueueSize   uint64
	reporter       *base.Reporter
	wg             sync.WaitGroup
	confined       bool
	rawQ           *tcp.Ring
	parsewg        sync.WaitGroup
	configs        map[utils.MyULID]conf.RELPSourceConfig
	forwarder      *ackForwarder
	parserEnv      *decoders.ParsersEnv
}

func NewRelpService(env *base.ProviderEnv) (base.Provider, error) {
	initRelpRegistry()
	s := RelpService{
		reporter:       env.Reporter,
		confined:       env.Confined,
		forwarder:      newAckForwarder(),
		configs:        make(map[utils.MyULID]conf.RELPSourceConfig),
		fatalErrorChan: make(chan struct{}),
	}
	s.StreamingService.init()
	s.StreamingService.BaseService.Logger = env.Logger.New("class", "RelpServer")
	s.StreamingService.BaseService.Binder = env.Binder
	s.StreamingService.handler = RelpHandler{Server: &s}
	s.StreamingService.confined = env.Confined
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

func (s *RelpService) Shutdown() {
	s.Stop()
}

func (s *RelpService) Start() ([]model.ListenerInfo, error) {
	infos := s.initTCPListeners()
	if len(infos) == 0 {
		s.Logger.Info("RELP service not started: no listener")
		return infos, nil
	}
	s.Logger.Info("Listening on RELP", "nb_services", len(infos))

	s.configs = make(map[utils.MyULID]conf.RELPSourceConfig, len(s.UnixListeners)+len(s.TCPListeners))
	for _, l := range s.UnixListeners {
		s.configs[l.Conf.ConfID] = conf.RELPSourceConfig(l.Conf)
	}
	for _, l := range s.TCPListeners {
		s.configs[l.Conf.ConfID] = conf.RELPSourceConfig(l.Conf)
	}

	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.parsewg.Add(1)
		go func() {
			// Parse() returns an error if something fatal happened
			err := s.Parse()
			s.parsewg.Done()
			if err != nil {
				s.Logger.Error(err.Error())
				s.dofatal()
			}
		}()
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.Listen()
	}()
	return infos, nil
}

func (s *RelpService) Stop() {
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
}

func (s *RelpService) SetConf(c conf.BaseConfig) {
	tcpConfigs := make([]conf.TCPSourceConfig, 0, len(c.RELPSource))
	for _, c := range c.RELPSource {
		tcpConfigs = append(tcpConfigs, conf.TCPSourceConfig(c))
	}
	s.StreamingService.SetConf(tcpConfigs, c.Parsers, c.Main.InputQueueSize, 132000)
	s.parserEnv = decoders.NewParsersEnv(c.Parsers, s.Logger)
	s.rawQ = tcp.NewRing(c.Main.InputQueueSize)
	s.ACKQueueSize = c.Main.InputQueueSize
}

func (s *RelpService) parseOne(raw *model.RawTCPMessage, gen *utils.Generator) error {
	syslogMsgs, err := s.parserEnv.Parse(&raw.Decoder, raw.Message)
	if err != nil {
		return err
	}

	for _, syslogMsg := range syslogMsgs {
		if syslogMsg == nil {
			continue
		}

		full := model.FullFactoryFrom(syslogMsg)
		full.Txnr = raw.Txnr
		full.ConfId = raw.ConfID
		full.Uid = gen.Uid()
		full.SourceType = "relp"
		full.ClientAddr = raw.Client
		full.SourcePort = int32(raw.LocalPort)
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

func (s *RelpService) Parse() error {
	gen := utils.NewGenerator()

	for {
		raw, err := s.rawQ.Get()
		if err != nil || raw == nil {
			return nil
		}

		err = s.parseOne(raw, gen)
		if err != nil {
			s.forwarder.ForwardFail(raw.ConnID, raw.Txnr)
			base.CountParsingError(base.RELP, raw.Client, raw.Decoder.Format)
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

func (s *RelpService) handleResponses(conn net.Conn, connID utils.MyULID, client string, logger log15.Logger) error {
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
					countRelpAnswer(client, 200)
				}
			} else if failures[next] {
				err = writeFailure(conn, next)
				if err == nil {
					failures[next] = false
					countRelpAnswer(client, 500)
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
	Server *RelpService
}

func (h RelpHandler) HandleConnection(conn net.Conn, c conf.TCPSourceConfig) (err error) {
	// http://www.rsyslog.com/doc/relp.html
	config := conf.RELPSourceConfig(c)
	s := h.Server
	s.AddConnection(conn)
	connID := s.forwarder.AddConn(s.ACKQueueSize)
	props := eprops(conn)
	l := makeLogger(s.Logger, props, "relp")
	l.Info("New client")
	defer l.Debug("Client gone away")
	clientCounter(base.RELP, props)

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
	scanner := utils.WithRecover(bufio.NewScanner(c))
	scanner.Split(utils.RelpSplit)
	scanner.Buffer(make([]byte, 0, 132000), 132000)

	for scanner.Scan() {
		splits = bytes.SplitN(scanner.Bytes(), sp, 3)
		txnr, err = utils.Atoi32(string(splits[0]))
		if err != nil {
			countRelpProtocolError(props.Client)
			return eerrors.Wrap(err, "Badly formed TXNR")
		}
		if txnr <= previous {
			countRelpProtocolError(props.Client)
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
				countRelpProtocolError(props.Client)
				return eerrors.Wrapf(err, "Unknown RELP command: %s", command)
			case fsm.InvalidEventError:
				countRelpProtocolError(props.Client)
				return eerrors.Wrapf(err, "Invalid RELP command: %s", command)
			case fsm.InternalError:
				countRelpProtocolError(props.Client)
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
					countRelpProtocolError(props.Client)
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
				incomingCounter(base.RELP, props)
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
