package store

import (
	"bytes"
	"context"
	"encoding/binary"
	"expvar"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/options"
	"github.com/dgraph-io/badger/y"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/db"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue"
	"github.com/stephane-martin/skewer/utils/waiter"
	"github.com/valyala/bytebufferpool"
	"go.uber.org/atomic"
)

var Registry *prometheus.Registry
var badgerGauge *prometheus.GaugeVec
var ackCounter *prometheus.CounterVec
var messageFilterCounter *prometheus.CounterVec
var retrieveTimeSummary prometheus.Summary
var lsmSize prometheus.GaugeFunc
var vlogSize prometheus.GaugeFunc

var once sync.Once

var msgsSlicePool = &sync.Pool{
	New: func() interface{} {
		return make([]*model.FullMessage, 0, 5000)
	},
}

var uidsPool = &sync.Pool{
	New: func() interface{} {
		return make([]utils.MyULID, 0, 5000)
	},
}

var compressPool bytebufferpool.Pool

func InitRegistry() {
	once.Do(func() {

		badgerGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "skw_store_entries_gauge",
				Help: "number of messages stored in the badger database",
			},
			[]string{"queue", "destination"},
		)

		ackCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_store_acks_total",
				Help: "number of ACKs received by the store",
			},
			[]string{"status", "destination"},
		)

		messageFilterCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_message_filtering_total",
				Help: "number of filtered messages by status",
			},
			[]string{"status", "client", "destination"},
		)

		retrieveTimeSummary = prometheus.NewSummary(
			prometheus.SummaryOpts{
				Help:       "histogram for the response time to retrieve messages from the Store",
				Name:       "skw_store_retrieve",
				Objectives: prometheus.DefObjectives,
				MaxAge:     prometheus.DefMaxAge,
				AgeBuckets: prometheus.DefAgeBuckets,
				BufCap:     prometheus.DefBufCap,
			},
		)

		lsmSize = prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Help: "Size of the key table in badger",
				Name: "skw_store_lsm_size",
			},
			getLSMSize,
		)

		vlogSize = prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Help: "Size of the value log in badger",
				Name: "skw_store_vlog_size",
			},
			getValueLogSize,
		)

		Registry = prometheus.NewRegistry()
		Registry.MustRegister(badgerGauge, ackCounter, messageFilterCounter, retrieveTimeSummary, lsmSize, vlogSize)
	})
}

func countACK(dest conf.DestinationType, status string) {
	ackCounter.WithLabelValues(status, conf.DestinationNames[dest]).Inc()
}

func countFiltered(dest conf.DestinationType, status string, client string) {
	messageFilterCounter.WithLabelValues(status, client, conf.DestinationNames[dest]).Inc()
}

type Destinations struct {
	atomic.Uint64
}

func (dests *Destinations) Store(ds conf.DestinationType) {
	dests.Uint64.Store(uint64(ds))
}

func (dests *Destinations) Load() (res []conf.DestinationType) {
	return conf.DestinationType(dests.Uint64.Load()).Iterate()
}

func (dests *Destinations) Has(one conf.DestinationType) bool {
	return conf.DestinationType(dests.Uint64.Load()).Has(one)
}

type QueueType uint8

const (
	Messages = iota
	Ready
	Sent
	Failed
	PermErrors
)

var Queues = map[QueueType]string{
	Ready:      "r",
	Sent:       "s",
	Failed:     "f",
	PermErrors: "p",
}

func getPartitionPrefix(qtype QueueType, dtype conf.DestinationType) string {
	return Queues[qtype] + conf.RDestinations[dtype]
}

type Backend struct {
	Partitions map[QueueType]map[conf.DestinationType]db.Partition
	Messages   db.Partition
	Configs    db.Partition
	Whole      db.Partition
}

func (b *Backend) GetPartition(qtype QueueType, dtype conf.DestinationType) db.Partition {
	return b.Partitions[qtype][dtype]
}

func NewBackend(parent *badger.DB, storeSecret *memguard.LockedBuffer) (b *Backend, err error) {
	b = new(Backend)
	b.Partitions = make(map[QueueType]map[conf.DestinationType]db.Partition, len(Queues))
	for qtype := range Queues {
		b.Partitions[qtype] = map[conf.DestinationType]db.Partition{}
		for _, dtype := range conf.Destinations {
			b.Partitions[qtype][dtype] = db.NewPartition(parent, getPartitionPrefix(qtype, dtype))
		}
	}
	b.Configs = db.NewPartition(parent, "co")
	b.Messages = db.NewPartition(parent, "ma")
	if storeSecret != nil {
		b.Messages, err = db.NewEncryptedPartition(b.Messages, storeSecret)
		if err != nil {
			return nil, eerrors.Wrap(err, "Failed to initialize encrypted partition")
		}
	}
	b.Whole = db.NewPartition(parent, "")
	return b, nil
}

type MessageStore struct {
	badger  *badger.DB
	backend *Backend
	count   *utils.RefCount

	wg        sync.WaitGroup
	purgeLock sync.Mutex
	dests     *Destinations

	ticker *time.Ticker
	logger log15.Logger

	closedChan     chan struct{}
	FatalErrorChan chan struct{}
	OutputsChans   map[conf.DestinationType]chan []*model.FullMessage
	zeroMsgFlags   map[conf.DestinationType]*atomic.Bool

	ackQueue        *queue.AckQueue
	nackQueue       *queue.AckQueue
	permerrorsQueue *queue.AckQueue

	confined        bool
	BatchSize       uint32
	addMissingMsgID bool
	generator       *utils.Generator
	uidsTmpBuf      []utils.MyULID
}

func (s *MessageStore) Confined() bool {
	return s.confined
}

func (s *MessageStore) Outputs(dest conf.DestinationType) chan []*model.FullMessage {
	return s.OutputsChans[dest]
}

func (s *MessageStore) Errors() chan struct{} {
	return s.FatalErrorChan
}

func (s *MessageStore) Destinations() []conf.DestinationType {
	return s.dests.Load()
}

func (s *MessageStore) SetDestinations(dests conf.DestinationType) {
	s.dests.Store(dests)
}

func (s *MessageStore) receiveAcks() error {
	var ackBatchSize = uint32(s.BatchSize * 4 / 5)
	var nackBatchSize = uint32(s.BatchSize / 10)
	acks := make([]queue.UidDest, 0, ackBatchSize)
	nacks := make([]queue.UidDest, 0, nackBatchSize)
	permerrs := make([]queue.UidDest, 0, nackBatchSize)
	for queue.WaitManyAckQueues(s.ackQueue, s.nackQueue, s.permerrorsQueue) {
		s.ackQueue.GetManyInto(&acks)
		s.nackQueue.GetManyInto(&nacks)
		s.permerrorsQueue.GetManyInto(&permerrs)
		err := s.doACK(acks)
		if err != nil {
			return eerrors.Wrap(err, "Error applying ACKs")
		}
		err = s.doNACK(nacks)
		if err != nil {
			return eerrors.Wrap(err, "Error applying NACKs")
		}
		err = s.doPermanentError(permerrs)
		if err != nil {
			return eerrors.Wrap(err, "Error applying PermErrors")
		}
	}
	return nil
}

/*
	if err == badger.ErrNoRoom {
		TODO: check that in another place
		store.logger.Crit("The store is full!")
		close(store.FatalErrorChan) // signal the caller service than we should stop everything
	} else {
		store.logger.Warn("Store unexpected error", "error", err)
	}
*/

func (s *MessageStore) tickResetFailures(ctx context.Context) error {
	for {
		select {
		case <-s.ticker.C:
			err := s.resetFailures()
			if err != nil {
				return err
			}
			err = s.PurgeBadger()
			if err != nil {
				s.logger.Warn("Error in the periodic badger purge", "error", err)
			}
		case <-ctx.Done():
			s.ticker.Stop()
			return nil
		}
	}
}

func (s *MessageStore) retrieveAndForward(ctx context.Context) (err error) {
	var wg sync.WaitGroup
	bucket := make(map[conf.DestinationType]*atomic.Value, len(conf.Destinations))

	var nilmsg []*model.FullMessage
	for _, d := range conf.Destinations {
		bucket[d] = new(atomic.Value)
		bucket[d].Store(nilmsg)
	}
	lctx, cancel := context.WithCancel(ctx)

	for _, d := range conf.Destinations {
		wg.Add(1)

		go func(dest conf.DestinationType) {
			defer wg.Done()

			ew := waiter.Default()
			var previousMsgs []*model.FullMessage

		ForwardLoop:
			for {
				if !s.dests.Has(dest) {
					// that destination is not currently selected, do nothing
					select {
					case <-lctx.Done():
						return
					case <-time.After(time.Second):
						continue ForwardLoop
					}
				}
				// look into the bucket for available messages
				msgs := bucket[dest].Load().([]*model.FullMessage)
				if len(msgs) == 0 {
					// no available message for that destination, let's wait a little
					select {
					case <-lctx.Done():
						return
					case <-time.After(ew.Next()):
						continue ForwardLoop
					}
				}

				// there are some messages to forward
				ew.Reset()
				select {
				case s.Outputs(dest) <- msgs:
					// s.Outputs() is a non-buffered chan. So when
					// s.Outputs() <- msgs returns, it means that the forwarder
					// has finished to process the previously provided messages.
					// Therefore we can now push back the previous messages slice
					// to the slice pool.
					if previousMsgs != nil {
						msgsSlicePool.Put(previousMsgs)
					}
					previousMsgs = msgs
					msgs = nil
					bucket[dest].Store(msgs)
				case <-lctx.Done():
					return
				}
			}
		}(d)
	}

	defer func() {
		cancel()
		wg.Wait()
		for _, d := range conf.Destinations {
			close(s.Outputs(d))
			// NACK the messages that were not delivered
			msgs := bucket[d].Load().([]*model.FullMessage)
			if len(msgs) > 0 {
				for _, message := range msgs {
					s.NACK(message.Uid, d)
					model.FullFree(message)
				}
				msgs = nil
				bucket[d].Store(msgs)
			}
		}
	}()

	next := make(map[conf.DestinationType]time.Time, len(conf.Destinations))
	waits := make(map[conf.DestinationType]*waiter.W, len(conf.Destinations))
	now := time.Now()
	for _, d := range conf.Destinations {
		waits[d] = waiter.Default()
		if s.dests.Has(d) {
			next[d] = now
		} else {
			next[d] = now.Add(time.Second)
		}
	}

	var currentDest conf.DestinationType
	var currentDestIdx uint
	var first time.Time

RetrieveLoop:
	for {
		select {
		case <-lctx.Done():
			return nil
		default:
		}
		// wait until we have something to do
		now = time.Now()
		first = now.Add(time.Hour)
		for _, d := range conf.Destinations {
			if next[d].Before(first) {
				first = next[d]
			}
		}
		if now.Before(first) {
			select {
			case <-lctx.Done():
				return nil
			case <-time.After(first.Sub(now)):
			}
		}

		currentDestIdx = (currentDestIdx + 1) % uint(len(conf.Destinations))
		currentDest = 1 << currentDestIdx

		if time.Now().Before(next[currentDest]) {
			// wait more for next check
			continue RetrieveLoop
		}
		if !s.dests.Has(currentDest) {
			// current destination is not selected
			next[currentDest] = now.Add(time.Second)
			continue RetrieveLoop
		}
		if len(bucket[currentDest].Load().([]*model.FullMessage)) > 0 {
			// previous messages are still there
			next[currentDest] = now.Add(waits[currentDest].Next())
			continue RetrieveLoop
		}
		if s.zeroMsgFlags[currentDest].Load() {
			// we are sure that there was no new message
			next[currentDest] = time.Now().Add(waits[currentDest].Next())
			continue RetrieveLoop
		}
		messages, err := s.retrieve(currentDest)
		if err != nil {
			return eerrors.Wrap(err, "Failed to retrieve messages from badger")
		}
		if len(messages) == 0 {
			// no messages in store for that destination
			s.zeroMsgFlags[currentDest].Store(true)
			next[currentDest] = time.Now().Add(waits[currentDest].Next())
			continue RetrieveLoop
		}
		waits[currentDest].Reset()
		bucket[currentDest].Store(messages)
	}
}

func (s *MessageStore) init(ctx context.Context) {
	lctx, cancel := context.WithCancel(ctx)

	s.FatalErrorChan = make(chan struct{})
	s.ticker = time.NewTicker(time.Minute)

	// only once, push back messages from previous run that may have been stuck in the sent queue
	s.logger.Debug("reset messages stuck in sent")
	err := s.resetStuckInSent()
	if err != nil {
		s.logger.Warn("Error resetting stuck sent messages", "error", err)
	}

	s.logger.Debug("reset failed messages")
	err = s.resetFailures()
	if err != nil {
		s.logger.Warn("Error resetting failed messages", "error", err)
	}

	// prune orphaned messages
	s.logger.Debug("prune orphaned messages")
	err = s.pruneOrphaned()
	if err != nil {
		s.logger.Warn("Error pruning orphaned messages", "error", err)
	}
	s.logger.Debug("prune orphaned messages done")

	s.initGauge()

	errs := make(chan error, 4)

	s.wg.Add(1)
	go func() {
		defer func() {
			s.logger.Debug("receiveAcks done")
			s.wg.Done()
		}()
		err := s.receiveAcks()
		if err != nil {
			errs <- err
		}
	}()

	s.wg.Add(1)
	go func() {
		defer func() {
			s.logger.Debug("tickResetFailures done")
			s.wg.Done()
		}()
		err := s.tickResetFailures(lctx)
		if err != nil {
			errs <- err
		}
	}()

	s.wg.Add(1)
	go func() {
		defer func() {
			s.logger.Debug("retrieveAndForward done")
			s.wg.Done()
		}()
		err := s.retrieveAndForward(lctx)
		if err != nil {
			errs <- err
		}
	}()

	go func() {
		// wait that we are asked to shutdown, or for an error in some of the goroutines
		select {
		case <-lctx.Done():
			// the goroutines tickResetFailures and retrieveAndForward have
			// been notified by the closing context, they will stop eventually
		case err := <-errs:
			s.logger.Error("Some error happened operating the Store. Shutting it down", "error", err)
			// notify the goroutines tickResetFailures and retrieveAndForward that
			// they must stop
			cancel()
			close(s.FatalErrorChan)
		}
		// dispose the queues. makes the goroutine receiveAcks stop.
		s.ackQueue.Dispose()
		s.nackQueue.Dispose()
		s.permerrorsQueue.Dispose()

		// wait that all goroutines have stopped
		s.logger.Debug("Waiting for the end of store goroutines")
		s.wg.Wait()
		s.logger.Debug("Final purge of badger")
		s.PurgeBadger()
		s.logger.Debug("Final close of badger")
		s.closeBadgers()
		// notify our caller
		close(s.closedChan)
	}()
}

func getLSMSize() float64 {
	size := int64(0)
	y.LSMSize.Do(
		func(kv expvar.KeyValue) {
			size = kv.Value.(*expvar.Int).Value()
		},
	)
	return float64(size)
}

func getValueLogSize() float64 {
	size := int64(0)
	y.VlogSize.Do(
		func(kv expvar.KeyValue) {
			size = kv.Value.(*expvar.Int).Value()
		},
	)
	return float64(size)
}

func NewStore(ctx context.Context, cfg conf.StoreConfig, r kring.Ring, dests conf.DestinationType, cfnd bool, l log15.Logger) (*MessageStore, error) {
	dirname := cfg.Dirname
	if cfnd {
		dirname = filepath.Join("/tmp", "store", dirname)
	}
	badgerOpts := badger.DefaultOptions
	badgerOpts.Dir = dirname
	badgerOpts.ValueDir = dirname
	badgerOpts.ValueThreshold = 32
	badgerOpts.MaxTableSize = cfg.MaxTableSize
	badgerOpts.SyncWrites = cfg.FSync
	badgerOpts.TableLoadingMode = options.LoadToRAM
	badgerOpts.ValueLogLoadingMode = options.MemoryMap
	badgerOpts.ValueLogFileSize = cfg.ValueLogFileSize
	badgerOpts.NumVersionsToKeep = 1

	err := os.MkdirAll(dirname, 0700)
	if err != nil {
		return nil, eerrors.Wrap(err, "failed to create the directory for the Store")
	}

	store := &MessageStore{
		confined:        cfnd,
		logger:          l.New("class", "MessageStore"),
		dests:           &Destinations{},
		BatchSize:       cfg.BatchSize,
		ackQueue:        queue.NewAckQueue(),
		nackQueue:       queue.NewAckQueue(),
		permerrorsQueue: queue.NewAckQueue(),
		closedChan:      make(chan struct{}),
		OutputsChans:    make(map[conf.DestinationType]chan []*model.FullMessage, len(conf.Destinations)),
		zeroMsgFlags:    make(map[conf.DestinationType]*atomic.Bool, len(conf.Destinations)),
		addMissingMsgID: cfg.AddMissingMsgID,
		generator:       utils.NewGenerator(),
		count:           utils.NewRefCount(),
	}
	store.dests.Store(dests)

	kv, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, eerrors.Wrap(err, "failed to open the badger database")
	}
	store.badger = kv

	var storeSecret *memguard.LockedBuffer
	if r != nil {
		sessionSecret, err := r.GetBoxSecret()
		if err != nil {
			return nil, eerrors.Wrap(err, "fail to retrieve the box secret")
		}
		defer sessionSecret.Destroy()
		storeSecret, err = cfg.GetSecretB(sessionSecret)
		if err != nil {
			return nil, eerrors.Wrap(err, "failed to retrieve the session secret")
		}
		if storeSecret != nil {
			store.logger.Info("The badger store is encrypted")
		}
	}
	store.backend, err = NewBackend(kv, storeSecret)
	if err != nil {
		return nil, eerrors.Wrap(err, "error creating the backend from the badger database")
	}

	for _, dest := range conf.Destinations {
		store.OutputsChans[dest] = make(chan []*model.FullMessage)
		store.zeroMsgFlags[dest] = atomic.NewBool(false)
	}

	store.wg.Add(1)
	go func() {
		defer store.wg.Done()
		store.init(ctx)
	}()

	return store, nil
}

func (s *MessageStore) WaitFinished() {
	<-s.closedChan
}

func (s *MessageStore) StoreAllSyslogConfigs(c conf.BaseConfig) (err error) {
	funcs := make([]utils.Func, 0, 20)

	for _, c := range c.TCPSource {
		tcpConf := c
		funcs = append(funcs, func() error {
			return s.StoreSyslogConfig(tcpConf.ConfID, tcpConf.FilterSubConfig)
		})
	}

	for _, c := range c.UDPSource {
		udpConf := c
		funcs = append(funcs, func() error {
			return s.StoreSyslogConfig(udpConf.ConfID, udpConf.FilterSubConfig)
		})
	}

	for _, c := range c.RELPSource {
		relpConf := c
		funcs = append(funcs, func() error {
			return s.StoreSyslogConfig(relpConf.ConfID, relpConf.FilterSubConfig)
		})
	}

	for _, c := range c.KafkaSource {
		kafkaConf := c
		funcs = append(funcs, func() error {
			return s.StoreSyslogConfig(kafkaConf.ConfID, kafkaConf.FilterSubConfig)
		})
	}

	for _, c := range c.GraylogSource {
		graylogConf := c
		funcs = append(funcs, func() error {
			return s.StoreSyslogConfig(graylogConf.ConfID, graylogConf.FilterSubConfig)
		})
	}

	funcs = append(funcs, func() error {
		return s.StoreSyslogConfig(c.Journald.ConfID, c.Journald.FilterSubConfig)
	})

	funcs = append(funcs, func() error {
		return s.StoreSyslogConfig(c.Accounting.ConfID, c.Accounting.FilterSubConfig)
	})

	funcs = append(funcs, func() error {
		return s.StoreSyslogConfig(c.MacOS.ConfID, c.MacOS.FilterSubConfig)
	})

	return utils.Chain(funcs...)
}

func (s *MessageStore) StoreSyslogConfig(confID utils.MyULID, config conf.FilterSubConfig) (err error) {
	txn := db.NewNTransaction(s.badger, true)
	defer txn.Discard()

	exists, err := s.backend.Configs.Exists(confID, txn)
	if err != nil {
		return eerrors.Wrap(err, "failed to check if some configuration is already stored in the dabatase")
	}
	if exists {
		return nil
	}
	err = s.backend.Configs.Set(confID, config.Export(), txn)
	if err != nil {
		return eerrors.Wrap(err, "failed to store some configuration in the database")
	}
	err = txn.Commit(nil)
	if err != nil {
		return eerrors.Wrap(err, "failed to commit after storing configuration in the database")
	}
	badgerGauge.WithLabelValues("syslogconf", "").Inc()
	return nil
}

func (s *MessageStore) GetSyslogConfig(confID utils.MyULID) (*conf.FilterSubConfig, error) {
	txn := db.NewNTransaction(s.badger, false)
	defer txn.Discard()
	data, err := s.backend.Configs.Get(confID, nil, txn)
	if err != nil {
		return nil, eerrors.Wrap(err, "failed to retrieve configuration from database")
	}
	if data == nil {
		return nil, eerrors.Errorf("unknown syslog configuration id: %s", confID.String())
	}
	c, err := conf.ImportSyslogConfig(data)
	if err != nil {
		return nil, eerrors.Wrap(err, "Failed to unmarshal configuration from the database")
	}
	return c, nil
}

func (s *MessageStore) initGauge() {
	s.logger.Debug("Calculating the store initial content size")
	defer s.logger.Debug("Done calculating the store initial content size")

	txn := db.NewNTransaction(s.badger, false)
	allkeys := s.backend.Whole.ListKeys(txn)
	txn.Discard()
	keysByPrefix := make(map[string][]utils.MyULID)
	var (
		wholekey, key, prefix string
		err                   error
		uid, k                utils.MyULID
	)
	for _, k = range allkeys {
		wholekey = string(k)
		if len(wholekey) == 18 {
			prefix = wholekey[:2]
			key = wholekey[2:]
			uid = utils.MyULID(key)
			keysByPrefix[prefix] = append(keysByPrefix[prefix], uid)
		}
	}

	badgerGauge.WithLabelValues("syslogconf", "").Set(float64(len(keysByPrefix[s.backend.Configs.Prefix()])))
	badgerGauge.WithLabelValues("messages", "").Set(float64(len(keysByPrefix[s.backend.Messages.Prefix()])))

	for dname, dtype := range conf.Destinations {
		badgerGauge.WithLabelValues("sent", dname).Set(0)

		prefix := getPartitionPrefix(Ready, dtype)
		uids := keysByPrefix[prefix]
		c := int(0)
		for _, uid = range uids {
			c++
			s.count.Inc(uid)
		}
		badgerGauge.WithLabelValues("ready", dname).Set(float64(c))

		prefix = getPartitionPrefix(Failed, dtype)
		uids = keysByPrefix[prefix]
		c = 0
		for _, uid = range uids {
			c++
			s.count.Inc(uid)
		}
		badgerGauge.WithLabelValues("failed", dname).Set(float64(c))

		prefix = getPartitionPrefix(PermErrors, dtype)
		uids = keysByPrefix[prefix]
		c = 0
		for _, uid = range uids {
			c++
			s.count.Inc(uid)
		}
		badgerGauge.WithLabelValues("permerrors", dname).Set(float64(c))
	}
}

func (s *MessageStore) closeBadgers() {
	err := s.badger.Close()
	if err != nil {
		s.logger.Warn("Error closing the badger", "error", err)
	}
	//s.logger.Debug("Badger databases are closed")
}

func (s *MessageStore) pruneOrphaned() error {
	var nb int
	var err error

	for {
		nb, err = prune(s.badger, s.backend)
		if err == nil {
			break
		}
		if err != badger.ErrConflict {
			return eerrors.Wrap(err, "Failed to prune orphaned messages")
		}
	}

	if nb > 0 {
		s.logger.Info("Successfully pruned orphaned messages", "nb", nb)
	}
	return nil
}

func prune(badg *badger.DB, bend *Backend) (nb int, err error) {
	txn := db.NewNTransaction(badg, true)
	defer txn.Discard()

	uids := bend.Messages.ListKeys(txn)
	orphaned := make([]utils.MyULID, 0)

L:
	for _, uid := range uids {
		for _, dest := range conf.Destinations {
			readyDB := bend.GetPartition(Ready, dest)
			have, err := readyDB.Exists(uid, txn)
			if err != nil {
				return 0, err
			}
			if have {
				continue L
			}
			failedDB := bend.GetPartition(Failed, dest)
			have, err = failedDB.Exists(uid, txn)
			if err != nil {
				return 0, err
			}
			if have {
				continue L
			}
			permDB := bend.GetPartition(PermErrors, dest)
			have, err = permDB.Exists(uid, txn)
			if err != nil {
				return 0, err
			}
			if have {
				continue L
			}
		}
		orphaned = append(orphaned, uid)
	}

	for _, uid := range orphaned {
		err = bend.Messages.Delete(uid, txn)
		if err != nil {
			return 0, err
		}
	}

	err = txn.Commit(nil)
	if err != nil {
		return 0, err
	}

	return len(orphaned), nil
}

func (s *MessageStore) resetStuckInSent() (err error) {
	// push back to "Ready" the messages that were sent out of the Store in the
	// last execution of skewer, but never were ACKed or NACKed
	var nb int
	for _, dest := range conf.Destinations {
	RetryLoop:
		for {
			nb, err = reset(s.badger, s.backend, dest)
			if err == badger.ErrConflict {
				continue RetryLoop
			}
			if err != nil {
				return eerrors.Wrap(err, "failed to reset messages stuck in the sent queue")
			}
			break RetryLoop
		}
		if nb > 0 {
			s.logger.Info("Reset messages from the sent queue", "dest", dest, "nb", nb)
		}
	}
	return nil
}

func reset(badg *badger.DB, bend *Backend, dest conf.DestinationType) (nb int, err error) {
	txn := db.NewNTransaction(badg, true)
	defer txn.Discard()

	sentDB := bend.GetPartition(Sent, dest)
	readyDB := bend.GetPartition(Ready, dest)

	nb, err = resetStuckInSentByDest(sentDB, readyDB, dest, txn)
	if err != nil {
		return 0, err
	}
	err = txn.Commit(nil)
	if err != nil {
		return 0, err
	}
	return nb, nil
}

func resetStuckInSentByDest(sentDB, readyDB db.Partition, dest conf.DestinationType, txn *db.NTransaction) (nb int, err error) {
	uids := sentDB.ListKeys(txn)
	err = sentDB.DeleteMany(uids, txn)
	if err != nil {
		return 0, err
	}
	for _, uid := range uids {
		err = readyDB.Set(uid, "true", txn)
		if err != nil {
			return 0, err
		}
	}
	return len(uids), nil
}

func (s *MessageStore) ReadAllBadgers() (map[string]string, map[string]string, map[string]string) {
	return nil, nil, nil // TODO
}

func (s *MessageStore) resetFailures() error {
	// push back messages from "failed" to "ready"
	for _, dest := range conf.Destinations {
		err := s.resetFailuresByDest(dest)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MessageStore) resetFailuresByDest(dest conf.DestinationType) (err error) {
	failedDB := s.backend.GetPartition(Failed, dest)
	readyDB := s.backend.GetPartition(Ready, dest)

	iterate := func(txn *db.NTransaction) (expiredUIDs []utils.MyULID, invalidUIDs []utils.MyULID) {
		expiredUIDs = make([]utils.MyULID, 0)
		invalidUIDs = make([]utils.MyULID, 0)

		// TODO: benchmark keyiterator vs keyvalueiterator ?
		iter := failedDB.KeyIterator(txn)
		defer iter.Close()

		now := time.Now()
		var timeb []byte
		var err error

		for iter.Rewind(); iter.Valid(); iter.Next() {
			uid := iter.Key()
			timeb, err = iter.Value(timeb)
			if err != nil {
				s.logger.Warn("Invalid entry in failed", "error", err)
				invalidUIDs = append(invalidUIDs, uid)
				continue
			}
			t, n := utils.Bytes2Time(timeb)
			if n <= 0 {
				s.logger.Warn("Invalid entry in failed")
				invalidUIDs = append(invalidUIDs, uid)
				continue
			}
			if now.Sub(t) >= time.Minute {
				// messages that failed to be delivered to Kafka should be tried again after 1 minute
				expiredUIDs = append(expiredUIDs, uid)
			}
		}
		return expiredUIDs, invalidUIDs
	}

	doReset := func() (nbExpired int, nbInvalid int, err error) {
		txn := db.NewNTransaction(s.badger, true)
		defer txn.Discard()

		expiredUIDs, invalidUIDs := iterate(txn)

		if len(invalidUIDs) == 0 && len(expiredUIDs) == 0 {
			return 0, 0, nil
		}

		if len(invalidUIDs) > 0 {
			err := failedDB.DeleteMany(invalidUIDs, txn)
			if err != nil {
				return 0, 0, eerrors.Wrap(err, "Failed to delete invalid entries")
			}
		}

		if len(expiredUIDs) == 0 {
			err := txn.Commit(nil)
			if err != nil {
				return 0, 0, err
			}
			return 0, len(invalidUIDs), nil
		}

		err = readyDB.AddManySame(expiredUIDs, "true", txn)
		if err != nil {
			return 0, 0, eerrors.Wrap(err, "Failed to push expired entries to the ready queue")
		}

		err = failedDB.DeleteMany(expiredUIDs, txn)
		if err != nil {
			return 0, 0, eerrors.Wrap(err, "Failed to delete expired entries from the failed queue")
		}

		err = txn.Commit(nil)
		if err != nil {
			return 0, 0, err
		}
		return len(expiredUIDs), len(invalidUIDs), nil
	}

	var nbExpired, nbInvalid int
	for {
		nbExpired, nbInvalid, err = doReset()
		if err != badger.ErrConflict {
			break
		}
	}
	if err != nil {
		return eerrors.Wrap(err, "failed to reset expired failures")
	}
	badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Add(float64(nbExpired))
	badgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Sub(float64(nbExpired + nbInvalid))
	if nbInvalid > 0 {
		s.logger.Info("Deleted some invalid entries", "nb", nbInvalid)
	}
	if nbExpired > 0 {
		s.logger.Debug("Pushed back expired failures to the ready queue", "nb", nbExpired)
	}
	return nil

}

func (s *MessageStore) PurgeBadger() (err error) {
	s.purgeLock.Lock()
	s.logger.Debug("Purge badger")
	defer func() {
		// run the purge
		e := s.badger.RunValueLogGC(0.25)
		if e != nil && e != badger.ErrNoRewrite {
			err = eerrors.Wrap(e, "Error happened when garbage collecting the badger")
		} else {
			s.logger.Debug("Purge done")
		}
		s.purgeLock.Unlock()
	}()

	// delete the messages that are not referenced anymore
	uids := s.count.GC()

	if len(uids) > 0 {
		for {
			txn := db.NewNTransaction(s.badger, true)
			err = s.backend.Messages.DeleteMany(uids, txn)
			if err == nil {
				break
			}
			if err != badger.ErrConflict {
				return err
			}
		}
		badgerGauge.WithLabelValues("messages", "").Sub(float64(len(uids)))
		for _, uid := range uids {
			s.count.Remove(uid)
		}
		s.logger.Debug("Purged messages from badger", "nb", len(uids))
	}
	return nil
}

func ingestReadyHelper(badg *badger.DB, readyDB db.Partition, queue map[utils.MyULID]string) error {
	txn := db.NewNTransaction(badg, true)
	defer txn.Discard()
	err := readyDB.AddManyTrueMap(queue, txn)
	if err != nil {
		return err
	}
	return txn.Commit(nil)
}

func ingestMsgsHelper(badg *badger.DB, msgsDB db.Partition, queue map[utils.MyULID]string) error {
	txn := db.NewNTransaction(badg, true)
	defer txn.Discard()
	err := msgsDB.AddMany(queue, txn)
	if err != nil {
		return err
	}
	return txn.Commit(nil)
}

func (s *MessageStore) ingestReadyByDest(queue map[utils.MyULID]string, dest conf.DestinationType) error {
	readyDB := s.backend.GetPartition(Ready, dest)
	for {
		err := ingestReadyHelper(s.badger, readyDB, queue)
		if err != badger.ErrConflict {
			return err
		}
	}
}

func (s *MessageStore) ingestMessages(queue map[utils.MyULID]string) error {
	for {
		err := ingestMsgsHelper(s.badger, s.backend.Messages, queue)
		if err != badger.ErrConflict {
			return err
		}
	}
}

func (s *MessageStore) Ingest(m map[utils.MyULID]string) (int, error) {
	length := len(m)

	if length == 0 {
		return 0, nil
	}
	w := snappy.NewBufferedWriter(ioutil.Discard)
	for k, v := range m {
		if len(v) == 0 {
			continue
		}
		cv := compressPool.Get()
		w.Reset(cv)
		_, _ = w.Write([]byte(v))
		w.Close()
		m[k] = cv.String()
		compressPool.Put(cv)
	}

	// first store the messages content in the messages db
	err := s.ingestMessages(m)
	if err != nil {
		return 0, err
	}
	badgerGauge.WithLabelValues("messages", "").Add(float64(length))

	// reference the new messages in the ready queue
	destinations := s.Destinations()
	var nbDone int32
	for _, dest := range destinations {
		err = s.ingestReadyByDest(m, dest)
		if err != nil {
			break
		}
		nbDone++
		s.zeroMsgFlags[dest].Store(false)
		badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Add(float64(length))
	}
	for msg := range m {
		s.count.New(msg, nbDone)
	}
	return length, err
}

func retrieveIterHelper(msgsDB, readyDB db.Partition, batchsize uint32, txn *db.NTransaction, l log15.Logger) (fUIDs []utils.MyULID, messages []*model.FullMessage, invalid []utils.MyULID, keysNotFound int) {
	messages = msgsSlicePool.Get().([]*model.FullMessage)[:0]
	allUIDs := uidsPool.Get().([]utils.MyULID)[:0]
	fUIDs = allUIDs[:0]
	invalid = make([]utils.MyULID, 0)

	var messageBytes []byte
	var err error

	protobuff := proto.NewBuffer(make([]byte, 0, 4096))

	// first iterate on ready keys
	iter := readyDB.KeyIterator(txn)
	fetched := uint32(0)
	for iter.Rewind(); fetched < batchsize && iter.Valid(); iter.Next() {
		allUIDs = append(allUIDs, iter.Key())
		fetched++
	}
	iter.Close()

	r := snappy.NewReader(nil)

	// fetch messages content and filter the UIDs
	for _, uid := range allUIDs {
		messageBytes, err = msgsDB.Get(uid, messageBytes, txn) // reuse or grow messageBytes at each step
		if err != nil {
			invalid = append(invalid, uid)
			keysNotFound++
			l.Debug("Error getting message content from message queue", "uid", uid.String(), "error", err)
			continue
		}
		if len(messageBytes) == 0 {
			invalid = append(invalid, uid)
			l.Debug("retrieved empty entry", "uid", uid)
			continue
		}

		r.Reset(bytes.NewReader(messageBytes))
		dec := compressPool.Get()
		_, err = dec.ReadFrom(r)

		if err != nil {
			invalid = append(invalid, uid)
			l.Debug("retrieved invalid compressed entry", "uid", uid, "message", "error", err)
			continue
		}

		protobuff.SetBuf(dec.Bytes())
		message, err := model.FromBuf(protobuff)
		compressPool.Put(dec)

		if err != nil {
			invalid = append(invalid, uid)
			l.Debug("retrieved invalid protobuf encoded entry", "uid", uid, "message", "error", err)
			continue
		}

		messages = append(messages, message)
		fUIDs = append(fUIDs, uid) // reuse allUIDs backing storage
	}
	return fUIDs, messages, invalid, keysNotFound
}

func tryRetrieveHelper(msgsDB, readyDB, sentDB db.Partition, badg *badger.DB, batchSize uint32, l log15.Logger) ([]utils.MyULID, []*model.FullMessage, int, int, error) {

	txn := db.NewNTransaction(badg, true)
	defer txn.Discard()
	var err error

	// fetch messages from badger
	uids, messages, invalidEntries, keysNotFound := retrieveIterHelper(msgsDB, readyDB, batchSize, txn, l)

	if len(invalidEntries) > 0 {
		l.Info("Found invalid entries", "number", len(invalidEntries))
		err = readyDB.DeleteMany(invalidEntries, txn)
		if err != nil {
			return nil, nil, 0, 0, eerrors.Wrap(err, "Error deleting invalid entries from 'ready' queue")
		}
		err = msgsDB.DeleteMany(invalidEntries, txn)
		if err != nil {
			return nil, nil, 0, 0, eerrors.Wrap(err, "Error deleting invalid entries from 'messages' queue")
		}
	}

	if len(uids) > 0 {
		err = sentDB.AddManySame(uids, "true", txn)
		if err != nil {
			return nil, nil, 0, 0, eerrors.Wrap(err, "Error copying messages to the 'sent' queue")
		}
		err = readyDB.DeleteMany(uids, txn)
		if err != nil {
			return nil, nil, 0, 0, eerrors.Wrap(err, "Error deleting messages from the 'ready' queue")
		}
	}

	return uids, messages, len(invalidEntries), keysNotFound, txn.Commit(nil)
}

func (s *MessageStore) retrieve(dest conf.DestinationType) ([]*model.FullMessage, error) {
	startt := time.Now()
	defer func() {
		retrieveTimeSummary.Observe(time.Since(startt).Seconds() * 1000)
	}()

	// messages object is allocated from a pool, so it's the responsability of
	// the caller to put back messages to the pool when finished
	messagesDB := s.backend.Messages
	readyDB := s.backend.GetPartition(Ready, dest)
	sentDB := s.backend.GetPartition(Sent, dest)

	var messages []*model.FullMessage
	var uids []utils.MyULID
	var nbInvalids int
	var nbNotFound int
	var err error

	for {
		uids, messages, nbInvalids, nbNotFound, err = tryRetrieveHelper(messagesDB, readyDB, sentDB, s.badger, s.BatchSize, s.logger)

		if err == nil {
			break
		}

		if messages != nil {
			msgsSlicePool.Put(messages)
		}
		if uids != nil {
			uidsPool.Put(uids)
		}

		if err != badger.ErrConflict {
			return nil, err
		}
	}

	badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Sub(float64(nbInvalids))
	badgerGauge.WithLabelValues("messages", conf.DestinationNames[dest]).Sub(float64(nbInvalids - nbNotFound))
	badgerGauge.WithLabelValues("sent", conf.DestinationNames[dest]).Add(float64(len(uids)))
	badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Sub(float64(len(uids)))

	if uids != nil {
		uidsPool.Put(uids)
	}
	return messages, nil
}

func (s *MessageStore) ACK(uid utils.MyULID, dest conf.DestinationType) {
	countACK(dest, "ack")
	_ = s.ackQueue.Put(uid, dest)
}

func doACKHelper(badg *badger.DB, bend *Backend, acks []queue.UidDest) (count map[conf.DestinationType]int, err error) {
	txn := db.NewNTransaction(badg, true)
	defer txn.Discard()

	count = make(map[conf.DestinationType]int)

	for _, ack := range acks {
		err = bend.GetPartition(Sent, ack.Dest).Delete(ack.Uid, txn)
		if err != nil {
			return nil, eerrors.Wrap(err, "Error removing messages from the Sent DB")
		}
		count[ack.Dest]++
	}
	return count, txn.Commit(nil)

}

func (s *MessageStore) doACK(acks []queue.UidDest) (err error) {
	if len(acks) == 0 {
		return
	}
	var count map[conf.DestinationType]int

	for {
		count, err = doACKHelper(s.badger, s.backend, acks)
		if err != badger.ErrConflict {
			break
		}
	}
	if err != nil {
		return err
	}

	for dtype, nb := range count {
		badgerGauge.WithLabelValues("sent", conf.DestinationNames[dtype]).Sub(float64(nb))
	}
	for _, ack := range acks {
		s.count.Dec(ack.Uid)
	}
	return nil
}

func (s *MessageStore) NACK(uid utils.MyULID, dest conf.DestinationType) {
	countACK(dest, "nack")
	_ = s.nackQueue.Put(uid, dest)
}

func doNACKHelper(badg *badger.DB, bend *Backend, nacks []queue.UidDest) (count map[conf.DestinationType]int, err error) {
	txn := db.NewNTransaction(badg, true)
	defer txn.Discard()

	count = make(map[conf.DestinationType]int)
	buf := string(utils.Time2Bytes(time.Now(), nil))

	for _, nack := range nacks {
		err = bend.GetPartition(Sent, nack.Dest).Delete(nack.Uid, txn)
		if err != nil {
			return nil, eerrors.Wrap(err, "Error removing messages from the Sent DB")
		}
		err = bend.GetPartition(Failed, nack.Dest).Set(nack.Uid, buf, txn)
		if err != nil {
			return nil, eerrors.Wrap(err, "Error moving message to the Failed DB")
		}
		count[nack.Dest]++
	}
	return count, txn.Commit(nil)
}

func (s *MessageStore) doNACK(nacks []queue.UidDest) (err error) {
	if len(nacks) == 0 {
		return
	}
	var count map[conf.DestinationType]int

	for {
		count, err = doNACKHelper(s.badger, s.backend, nacks)
		if err != badger.ErrConflict {
			break
		}
	}
	if err != nil {
		return err
	}

	for dtype, nb := range count {
		badgerGauge.WithLabelValues("failed", conf.DestinationNames[dtype]).Add(float64(nb))
		badgerGauge.WithLabelValues("sent", conf.DestinationNames[dtype]).Sub(float64(nb))
	}
	return nil
}

func (s *MessageStore) PermError(uid utils.MyULID, dest conf.DestinationType) {
	countACK(dest, "permerror")
	_ = s.permerrorsQueue.Put(uid, dest)
}

func doPermErrorHelper(badg *badger.DB, bend *Backend, nacks []queue.UidDest) (count map[conf.DestinationType]int, err error) {
	txn := db.NewNTransaction(badg, true)
	defer txn.Discard()

	count = make(map[conf.DestinationType]int)
	timeb := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(timeb, time.Now().UnixNano())
	buf := string(timeb[:n])

	for _, nack := range nacks {
		err = bend.GetPartition(Sent, nack.Dest).Delete(nack.Uid, txn)
		if err != nil {
			return nil, eerrors.Wrap(err, "Error removing messages from the Sent DB")
		}
		err = bend.GetPartition(PermErrors, nack.Dest).Set(nack.Uid, buf, txn)
		if err != nil {
			return nil, eerrors.Wrap(err, "Error moving message to the PermErrors DB")
		}
		count[nack.Dest]++
	}
	return count, txn.Commit(nil)
}

func (s *MessageStore) doPermanentError(pes []queue.UidDest) (err error) {
	if len(pes) == 0 {
		return
	}
	var count map[conf.DestinationType]int

	for {
		count, err = doPermErrorHelper(s.badger, s.backend, pes)
		if err != badger.ErrConflict {
			break
		}
	}
	if err != nil {
		return err
	}

	for dtype, nb := range count {
		badgerGauge.WithLabelValues("permerrors", conf.DestinationNames[dtype]).Add(float64(nb))
		badgerGauge.WithLabelValues("sent", conf.DestinationNames[dtype]).Sub(float64(nb))
	}
	return nil
}
