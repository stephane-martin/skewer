package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awnumar/memguard"
	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/options"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/db"
	"github.com/stephane-martin/skewer/utils/queue"
)

var Registry *prometheus.Registry
var badgerGauge *prometheus.GaugeVec
var ackCounter *prometheus.CounterVec
var messageFilterCounter *prometheus.CounterVec
var once sync.Once

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

		Registry = prometheus.NewRegistry()
		Registry.MustRegister(badgerGauge, ackCounter, messageFilterCounter)

		// TODO: gather badger metrics

		/*
			expvarCollector := prometheus.NewExpvarCollector(map[string]*prometheus.Desc{
				"memstats": prometheus.NewDesc(
					"store_memstats",
					"All numeric memstats as one metric family. Not a good role-model, actually... ;-)",
					[]string{"type"}, nil,
				),
			})
		*/
		//Registry.MustRegister(badgerGauge, ackCounter, expvarCollector)
		//Registry.MustRegister(badgerGauge, ackCounter, prometheus.NewGoCollector())
		//Registry.MustRegister(badgerGauge, ackCounter, prometheus.NewProcessCollector(os.Getpid(), "store"))
	})
}

type Destinations struct {
	d uint64
}

func (dests *Destinations) Store(ds conf.DestinationType) {
	atomic.StoreUint64(&dests.d, uint64(ds))
}

func (dests *Destinations) Load() (res []conf.DestinationType) {
	return conf.DestinationType(atomic.LoadUint64(&dests.d)).Iterate()
}

func (dests *Destinations) Has(one conf.DestinationType) bool {
	return conf.DestinationType(atomic.LoadUint64(&dests.d)).Has(one)
}

type QueueType uint8

const (
	Messages = iota
	Ready
	Sent
	Failed
	PermErrors
)

var Queues = map[QueueType]byte{
	Messages:   'm',
	Ready:      'r',
	Sent:       's',
	Failed:     'f',
	PermErrors: 'p',
}

func getPartitionPrefix(qtype QueueType, dtype conf.DestinationType) (res []byte) {
	res = make([]byte, 2)
	res[0] = Queues[qtype]
	res[1] = conf.RDestinations[dtype]
	return res
}

type Backend struct {
	Partitions map[QueueType](map[conf.DestinationType]db.Partition)
}

func (b *Backend) GetPartition(qtype QueueType, dtype conf.DestinationType) db.Partition {
	return (b.Partitions[qtype])[dtype]
}

func NewBackend(parent *badger.DB, storeSecret *memguard.LockedBuffer) *Backend {
	b := Backend{}
	b.Partitions = map[QueueType](map[conf.DestinationType]db.Partition){}
	for qtype := range Queues {
		b.Partitions[qtype] = map[conf.DestinationType]db.Partition{}
		for _, dtype := range conf.Destinations {
			(b.Partitions[qtype])[dtype] = db.NewPartition(parent, getPartitionPrefix(qtype, dtype))
		}
	}
	if storeSecret != nil {
		for _, dtype := range conf.Destinations {
			b.Partitions[Messages][dtype] = db.NewEncryptedPartition(b.Partitions[Messages][dtype], storeSecret)
		}
	}
	return &b
}

type MessageStore struct {
	badger          *badger.DB
	backend         *Backend
	syslogConfigsDB db.Partition

	wg    sync.WaitGroup
	dests *Destinations

	ticker *time.Ticker
	logger log15.Logger

	closedChan     chan struct{}
	FatalErrorChan chan struct{}
	OutputsChans   map[conf.DestinationType](chan []*model.FullMessage)

	toStashQueue    *queue.MessageQueue
	ackQueue        *queue.AckQueue
	nackQueue       *queue.AckQueue
	permerrorsQueue *queue.AckQueue

	confined        bool
	batchSize       uint32
	addMissingMsgID bool
	generator       *utils.Generator
	msgsSlicePool   *sync.Pool
	uidsSlicePool   *sync.Pool
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

func (s *MessageStore) receiveAcks() {
	defer s.wg.Done()
	var ackBatchSize uint32 = s.batchSize * 4 / 5
	var nackBatchSize uint32 = s.batchSize / 10
	acks := make([]queue.UidDest, 0, ackBatchSize)
	nacks := make([]queue.UidDest, 0, nackBatchSize)
	permerrs := make([]queue.UidDest, 0, nackBatchSize)
	for queue.WaitManyAckQueues(s.ackQueue, s.nackQueue, s.permerrorsQueue) {
		s.ackQueue.GetManyInto(&acks)
		s.nackQueue.GetManyInto(&nacks)
		s.permerrorsQueue.GetManyInto(&permerrs)
		s.doACK(acks)
		s.doNACK(nacks)
		s.doPermanentError(permerrs)
	}
}

func (s *MessageStore) cleanup(ctx context.Context) {
	<-ctx.Done()
	s.toStashQueue.Dispose()
	s.ackQueue.Dispose()
	s.nackQueue.Dispose()
	s.permerrorsQueue.Dispose()

	s.wg.Done()
	s.wg.Wait()
	s.PurgeBadger()
	s.closeBadgers()
	close(s.closedChan)
}

func (s *MessageStore) consumeStashQueue() {
	defer s.wg.Done()
	var err error
	messages := make([]*model.FullMessage, 0, s.batchSize)
	messagesMap := make(map[utils.MyULID]([]byte), s.batchSize)
	for s.toStashQueue.Wait(0) {
		s.toStashQueue.GetManyInto(&messages)
		if len(messages) == 0 {
			continue
		}
		_, err = s.ingest(messages, &messagesMap)
		if err != nil {
			s.logger.Warn("Ingestion error", "error", err)
		}
		for _, m := range messages {
			model.FullFree(m)
		}
	}
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

func (s *MessageStore) tickResetFailures(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-s.ticker.C:
			err := s.resetFailures()
			if err != nil {
				s.logger.Warn("Error resetting failures", "error", err)
			}
		case <-ctx.Done():
			s.ticker.Stop()
			//store.logger.Debug("Store ticker has been stopped")
			return
		}
	}
}

func (s *MessageStore) forward(ctx context.Context, d conf.DestinationType) {
	var wg sync.WaitGroup
	c := s.OutputsChans[d]
	doneChan := ctx.Done()

	defer func() {
		wg.Wait()
		close(c)
		s.wg.Done()
	}()

	var messages []*model.FullMessage
	var oldMessages []*model.FullMessage
	for {

	wait_messages:
		for {
			select {
			case <-doneChan:
				return
			default:
				if s.dests.Has(d) {
					messages = s.retrieve(d)
					if len(messages) > 0 {
						break wait_messages
					} else {
						select {
						case <-doneChan:
							return
						case <-time.After(time.Second):
						}
					}
				} else {
					// if the current destination is not active, we avoid to query the badger database
					select {
					case <-doneChan:
						return
					case <-time.After(time.Second):
					}
				}
			}
		}

		select {
		case s.Outputs(d) <- messages:
			// at that point, the destination has finished processing oldMessages
			if oldMessages != nil {
				s.msgsSlicePool.Put(oldMessages)
				oldMessages = messages
			}
		case <-doneChan:
			// TODO: nack the messages?
			return
		}

	}
}

func (s *MessageStore) init(ctx context.Context) {
	defer s.wg.Done()
	s.FatalErrorChan = make(chan struct{})
	s.ticker = time.NewTicker(time.Minute)

	// only once, push back messages from previous run that may have been stuck in the sent queue
	s.logger.Debug("reset messages stuck in sent")
	err := s.resetStuckInSent()
	if err != nil {
		s.logger.Warn("Error resetting stuck sent messages", "errot", err)
	}

	// prune orphaned messages
	s.logger.Debug("prune orphaned messages")
	err = s.pruneOrphaned()
	if err != nil {
		s.logger.Warn("Error pruning orphaned messages", "error", err)
	}

	err = s.initGauge()
	if err != nil {
		s.logger.Warn("Error calculating initial store metrics", "error", err)
	}

	s.wg.Add(1)
	go s.receiveAcks()

	s.wg.Add(1)
	go s.cleanup(ctx)

	s.wg.Add(1)
	go s.consumeStashQueue()

	s.wg.Add(1)
	go s.tickResetFailures(ctx)

	// TODO: optimize. only active destinations should be forwarded to.
	for _, dest := range conf.Destinations {
		s.wg.Add(1)
		go s.forward(ctx, dest)
	}
}

func NewStore(ctx context.Context, cfg conf.StoreConfig, r kring.Ring, dests conf.DestinationType, cfnd bool, l log15.Logger) (*MessageStore, error) {
	dirname := cfg.Dirname
	if cfnd {
		dirname = filepath.Join("/tmp", "store", dirname)
	}
	badgerOpts := badger.DefaultOptions
	badgerOpts.Dir = dirname
	badgerOpts.ValueDir = dirname
	badgerOpts.MaxTableSize = cfg.MaxTableSize
	badgerOpts.SyncWrites = cfg.FSync
	badgerOpts.TableLoadingMode = options.MemoryMap
	badgerOpts.ValueLogLoadingMode = options.MemoryMap
	badgerOpts.ValueLogFileSize = cfg.ValueLogFileSize

	err := os.MkdirAll(dirname, 0700)
	if err != nil {
		return nil, err
	}

	store := &MessageStore{
		confined:        cfnd,
		logger:          l.New("class", "MessageStore"),
		dests:           &Destinations{},
		batchSize:       cfg.BatchSize,
		toStashQueue:    queue.NewMessageQueue(),
		ackQueue:        queue.NewAckQueue(),
		nackQueue:       queue.NewAckQueue(),
		permerrorsQueue: queue.NewAckQueue(),
		closedChan:      make(chan struct{}),
		OutputsChans:    make(map[conf.DestinationType](chan []*model.FullMessage)),
		addMissingMsgID: cfg.AddMissingMsgID,
		generator:       utils.NewGenerator(),
		msgsSlicePool: &sync.Pool{
			New: func() interface{} {
				return make([]*model.FullMessage, 0, cfg.BatchSize)
			},
		},
		uidsSlicePool: &sync.Pool{
			New: func() interface{} {
				return make([]utils.MyULID, 0, cfg.BatchSize)
			},
		},
	}

	store.dests.Store(dests)

	kv, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, err
	}
	store.badger = kv

	var storeSecret *memguard.LockedBuffer
	if r != nil {
		sessionSecret, err := r.GetBoxSecret()
		if err != nil {
			return nil, err
		}
		defer sessionSecret.Destroy()
		storeSecret, err = cfg.GetSecretB(sessionSecret)
		if err != nil {
			return nil, err
		}
		if storeSecret != nil {
			store.logger.Info("The badger store is encrypted")
		}
	}
	store.backend = NewBackend(kv, storeSecret)
	store.syslogConfigsDB = db.NewPartition(kv, []byte("configs"))

	for _, dest := range conf.Destinations {
		store.OutputsChans[dest] = make(chan []*model.FullMessage)
	}

	store.wg.Add(1)
	go store.init(ctx)

	return store, nil
}

func (s *MessageStore) WaitFinished() {
	<-s.closedChan
}

func (s *MessageStore) StoreAllSyslogConfigs(c conf.BaseConfig) (err error) {
	funcs := []utils.Func{}

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

	return utils.Chain(funcs...)
}

func (s *MessageStore) StoreSyslogConfig(confID utils.MyULID, config conf.FilterSubConfig) error {
	data := config.Export()
	exists, err := s.syslogConfigsDB.Exists(confID, nil)
	if err != nil {
		return err
	}
	if !exists {
		err = s.syslogConfigsDB.Set(confID, data, nil)
		if err != nil {
			return err
		}
		badgerGauge.WithLabelValues("syslogconf", "").Inc()
	}
	return nil
}

func (s *MessageStore) GetSyslogConfig(confID utils.MyULID) (*conf.FilterSubConfig, error) {
	data, err := s.syslogConfigsDB.Get(confID, nil)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("Unknown syslog configuration id")
	}
	c, err := conf.ImportSyslogConfig(data)
	if err != nil {
		return nil, fmt.Errorf("Can't unmarshal the syslog config: %s", err.Error())
	}
	return c, nil
}

func (s *MessageStore) initGauge() error {
	badgerGauge.WithLabelValues("syslogconf", "").Set(float64(s.syslogConfigsDB.Count(nil)))
	return s.badger.View(func(txn *badger.Txn) error {
		for dname, dtype := range conf.Destinations {
			badgerGauge.WithLabelValues("messages", dname).Set(float64(s.backend.GetPartition(Messages, dtype).Count(txn)))
			badgerGauge.WithLabelValues("ready", dname).Set(float64(s.backend.GetPartition(Ready, dtype).Count(txn)))
			badgerGauge.WithLabelValues("sent", dname).Set(float64(s.backend.GetPartition(Sent, dtype).Count(txn)))
			badgerGauge.WithLabelValues("failed", dname).Set(float64(s.backend.GetPartition(Failed, dtype).Count(txn)))
			badgerGauge.WithLabelValues("permerrors", dname).Set(float64(s.backend.GetPartition(PermErrors, dtype).Count(txn)))
		}
		return nil
	})
}

func (s *MessageStore) closeBadgers() {
	err := s.badger.Close()
	if err != nil {
		s.logger.Warn("Error closing the badger", "error", err)
	}
	//s.logger.Debug("Badger databases are closed")
}

func (s *MessageStore) pruneOrphaned() (err error) {
	txn := s.badger.NewTransaction(true)
	defer txn.Discard()

	for _, dest := range conf.Destinations {
		err = s.pruneOrphanedByDest(dest, txn)
		if err != nil {
			return
		}
	}

	err = txn.Commit(nil)
	if err == badger.ErrConflict {
		return s.pruneOrphaned()
	} else if err != nil {
		s.logger.Warn("Error commiting the deletion of orphaned messages", "error", err)
	} else {
		s.logger.Info("Pruned orphaned messages")
	}
	return
}

func (s *MessageStore) pruneOrphanedByDest(dest conf.DestinationType, txn *badger.Txn) (err error) {
	// find if we have some old full messages
	var have bool
	messagesDB := s.backend.GetPartition(Messages, dest)
	readyDB := s.backend.GetPartition(Ready, dest)
	failedDB := s.backend.GetPartition(Failed, dest)
	permDB := s.backend.GetPartition(PermErrors, dest)

	uids := messagesDB.ListKeys(txn)

	// check if the corresponding uid exists in "ready" or "failed" or "permerrors"
	orphanedUIDs := []utils.MyULID{}
	for _, uid := range uids {
		have, err = readyDB.Exists(uid, txn)
		if err != nil {
			return
		}
		if have {
			continue
		}
		have, err = failedDB.Exists(uid, txn)
		if err != nil {
			return
		}
		if have {
			continue
		}
		have, err = permDB.Exists(uid, txn)
		if err != nil {
			return
		}
		if have {
			continue
		}
		orphanedUIDs = append(orphanedUIDs, uid)
	}

	// if no match, delete the message
	for _, uid := range orphanedUIDs {
		err = messagesDB.Delete(uid, txn)
		if err != nil {
			s.logger.Warn("Error deleting orphaned messages", "error", err)
			return
		}
	}

	return
}

func (s *MessageStore) resetStuckInSent() (err error) {
	// push back to "Ready" the messages that were sent out of the Store in the
	// last execution of skewer, but never were ACKed or NACKed
	txn := s.badger.NewTransaction(true)
	defer txn.Discard()

	for _, dest := range conf.Destinations {
		err = s.resetStuckInSentByDest(dest, txn)
		if err != nil {
			return
		}
	}

	err = txn.Commit(nil)
	if err == badger.ErrConflict {
		// retry
		return s.resetStuckInSent()
	} else if err != nil {
		s.logger.Warn("Error commiting stuck messages", "error", err)
	} else {
		s.logger.Info("Pushed back stuck messages from sent to ready")
	}
	return

}

func (s *MessageStore) resetStuckInSentByDest(dest conf.DestinationType, txn *badger.Txn) (err error) {
	sentDB := s.backend.GetPartition(Sent, dest)
	readyDB := s.backend.GetPartition(Ready, dest)

	uids := sentDB.ListKeys(txn)
	err = sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error deleting stuck messages from the sent queue", "error", err)
		return
	}
	for _, uid := range uids {
		err = readyDB.Set(uid, []byte("true"), txn)
		if err != nil {
			s.logger.Warn("Error moving stuck messages from the sent queue to the ready queue", "error", err)
			return
		}
	}
	return
}

func (s *MessageStore) ReadAllBadgers() (map[string]string, map[string]string, map[string]string) {
	return nil, nil, nil // TODO
}

func (s *MessageStore) resetFailures() (err error) {
	// push back messages from "failed" to "ready"
	for _, dest := range conf.Destinations {
		err = s.resetFailuresByDest(dest)
		if err != nil {
			return err
		}
	}
	s.PurgeBadger()
	return nil
}

func (s *MessageStore) resetFailuresByDest(dest conf.DestinationType) (err error) {
	var t time.Time
	failedDB := s.backend.GetPartition(Failed, dest)
	readyDB := s.backend.GetPartition(Ready, dest)

	for {
		//lok := s.readyMutexes[dest]
		//cond := s.availConditions[dest]
		txn := s.badger.NewTransaction(true)
		now := time.Now()
		iter := failedDB.KeyValueIterator(s.batchSize/5, txn)
		uids := []utils.MyULID{}
		invalidUids := []utils.MyULID{}

		for iter.Rewind(); iter.Valid(); iter.Next() {
			uid := iter.Key()
			time_s := string(iter.Value())
			t, err = time.Parse(time.RFC3339, time_s)
			if err == nil {
				if now.Sub(t) >= time.Minute {
					// messages that failed to be delivered to Kafka should be tried again after 1 minute
					uids = append(uids, uid)
				}
			} else {
				s.logger.Warn("Invalid entry in failed", "wrong_timestamp", time_s)
				invalidUids = append(invalidUids, uid)
			}
		}
		iter.Close()

		if len(invalidUids) > 0 {
			s.logger.Info("Found invalid entries in 'failed'", "number", len(invalidUids))
			err = failedDB.DeleteMany(invalidUids, txn)
			if err != nil {
				s.logger.Warn("Error deleting invalid entries", "error", err)
				txn.Discard()
				return err
			}
		}

		if len(uids) == 0 {
			if len(invalidUids) > 0 {
				err = txn.Commit(nil)
				if err == nil {
					badgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Sub(float64(len(invalidUids)))
				}
				return err
			}
			txn.Discard()
			return nil
		}

		err = readyDB.AddManySame(uids, []byte("true"), txn)
		if err != nil {
			s.logger.Warn("Error pushing entries from failed queue to ready queue", "error", err)
			txn.Discard()
			return err
		}

		err = failedDB.DeleteMany(uids, txn)
		if err != nil {
			s.logger.Warn("Error deleting entries from failed queue", "error", err)
			txn.Discard()
			return err
		}

		err = txn.Commit(nil)
		if err == nil {
			badgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Sub(float64(len(invalidUids)))
			badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Add(float64(len(uids)))
			badgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Sub(float64(len(uids)))
		} else {
			s.logger.Warn("Error commiting resetFailures", "error", err)
			return err
		}
	}
}

func (s *MessageStore) PurgeBadger() {
	err := s.badger.PurgeOlderVersions()
	if err == nil {
		err = s.badger.RunValueLogGC(0.5)
		if err != nil {
			s.logger.Info("Error garbage collecting badger", "error", err)
		}
	} else {
		s.logger.Info("Error purging badger", "error", err)
	}
}

func (s *MessageStore) Stash(m *model.FullMessage) (fatal error, nonfatal error) {
	fatal = s.toStashQueue.Put(m)
	return fatal, nil
}

func (s *MessageStore) ingestByDest(queue map[utils.MyULID]([]byte), dest conf.DestinationType, txn *badger.Txn) (err error) {
	messagesDB := s.backend.GetPartition(Messages, dest)
	readyDB := s.backend.GetPartition(Ready, dest)

	err = messagesDB.AddMany(queue, txn)
	if err != nil {
		return err
	}
	return readyDB.AddManyTrueMap(queue, txn)
}

func (s *MessageStore) ingest(queue []*model.FullMessage, messagesMap *(map[utils.MyULID]([]byte))) (n int, err error) {
	if len(queue) == 0 {
		return 0, nil
	}
	var b []byte
	var m *model.FullMessage

	// clear the map that will hold the marshalled bytes
	var uid utils.MyULID
	for uid = range *messagesMap {
		delete(*messagesMap, uid)
	}

	for _, m = range queue {
		if s.addMissingMsgID && len(m.Fields.MsgId) == 0 {
			m.Fields.MsgId = s.generator.Uid().String()
		}
		b, err = m.Marshal()
		if err == nil {
			if len(b) == 0 {
				s.logger.Warn("Ingestion of empty message", "uid", m.Uid)
			} else {
				(*messagesMap)[m.Uid] = b
			}
		} else {
			s.logger.Warn("Discarded a message that could not be marshaled", "error", err)
		}
	}

	length := len(*messagesMap)
	if length == 0 {
		return 0, nil
	}

	dests := s.Destinations()

	txn := s.badger.NewTransaction(true)
	defer txn.Discard()

	for _, dest := range dests {
		err = s.ingestByDest(*messagesMap, dest, txn)
		if err != nil {
			return 0, err
		}
	}

	err = txn.Commit(nil)
	if err == nil {
		for _, dest := range dests {
			badgerGauge.WithLabelValues("messages", conf.DestinationNames[dest]).Add(float64(length))
			badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Add(float64(length))
		}
		return length, nil
	} else if err == badger.ErrConflict {
		return s.ingest(queue, messagesMap)
	} else {
		return 0, err
	}

}

func (s *MessageStore) retrieve(dest conf.DestinationType) (messages []*model.FullMessage) {
	txn := s.badger.NewTransaction(true)
	defer txn.Discard()

	readyDB := s.backend.GetPartition(Ready, dest)
	messagesDB := s.backend.GetPartition(Messages, dest)
	sentDB := s.backend.GetPartition(Sent, dest)

	messages = s.msgsSlicePool.Get().([]*model.FullMessage)[:0]
	uids := s.uidsSlicePool.Get().([]utils.MyULID)[:0]
	defer s.uidsSlicePool.Put(uids)

	iter := readyDB.KeyIterator(s.batchSize, txn)
	invalidEntries := make([]utils.MyULID, 0)
	var message *model.FullMessage
	var keysNotFound int
	var uid utils.MyULID
	var fetched uint32
	var err error
	var messageBytes []byte

	for iter.Rewind(); iter.Valid() && fetched < s.batchSize; iter.Next() {
		uid = iter.Key()
		messageBytes, err = messagesDB.Get(uid, txn)
		if err != nil {
			invalidEntries = append(invalidEntries, uid)
			keysNotFound++
			s.logger.Warn("Error getting message content from message queue", "uid", uid, "dest", dest, "error", err)
			continue
		}
		if len(messageBytes) == 0 {
			invalidEntries = append(invalidEntries, uid)
			s.logger.Warn("retrieved empty entry", "uid", uid)
			continue
		}
		message = model.FullFactory()
		err = message.Unmarshal(messageBytes)
		if err != nil {
			invalidEntries = append(invalidEntries, uid)
			s.logger.Warn("retrieved invalid entry", "uid", uid, "message", string(messageBytes), "dest", dest, "error", err)
			continue
		}
		messages = append(messages, message)
		uids = append(uids, uid)
		fetched++
	}
	iter.Close()

	if len(invalidEntries) > 0 {
		s.logger.Info("Found invalid entries", "number", len(invalidEntries))
		err := readyDB.DeleteMany(invalidEntries, txn)
		if err != nil {
			s.logger.Warn("Error deleting invalid entries from 'ready' queue", "error", err)
			return nil
		}
		err = messagesDB.DeleteMany(invalidEntries, txn)
		if err != nil {
			s.logger.Warn("Error deleting invalid entries from 'messages' queue", "error", err)
			return nil
		}
	}

	if len(messages) == 0 {
		if len(invalidEntries) > 0 {
			err := txn.Commit(nil)
			if err == nil {
				badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Sub(float64(len(invalidEntries)))
				badgerGauge.WithLabelValues("messages", conf.DestinationNames[dest]).Sub(float64(len(invalidEntries) - keysNotFound))
			}
		}
		return nil
	}

	err = sentDB.AddManySame(uids, []byte("true"), txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the 'sent' queue", "error", err)
		return nil
	}
	err = readyDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error deleting messages from the 'ready' queue", "error", err)
		return nil
	}

	err = txn.Commit(nil)
	if err == nil {
		badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Sub(float64(len(invalidEntries)))
		badgerGauge.WithLabelValues("messages", conf.DestinationNames[dest]).Sub(float64(len(invalidEntries) - keysNotFound))
		badgerGauge.WithLabelValues("sent", conf.DestinationNames[dest]).Add(float64(len(uids)))
		badgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Sub(float64(len(uids)))
		return messages
	} else if err == badger.ErrConflict {
		// retry
		s.msgsSlicePool.Put(messages)
		return s.retrieve(dest)
	} else {
		s.logger.Warn("Error committing to badger in retrieve", "error", err)
		return nil
	}

}

func sortAck(acks []queue.UidDest) (res map[conf.DestinationType]([]utils.MyULID)) {
	// TODO: optimize
	var ok bool
	res = map[conf.DestinationType]([]utils.MyULID){}
	for _, ack := range acks {
		if _, ok = res[ack.Dest]; !ok {
			res[ack.Dest] = make([]utils.MyULID, 0, len(acks))
		}
		res[ack.Dest] = append(res[ack.Dest], ack.Uid)
	}
	return res
}

func (s *MessageStore) ACK(uid utils.MyULID, dest conf.DestinationType) {
	ackCounter.WithLabelValues("ack", conf.DestinationNames[dest]).Inc()
	_ = s.ackQueue.Put(uid, dest)
}

func (s *MessageStore) doACK(acks []queue.UidDest) {
	m := sortAck(acks)
	for desttype := range m {
		s.ackByDest(m[desttype], desttype)
	}
}

func (s *MessageStore) doNACK(nacks []queue.UidDest) {
	m := sortAck(nacks)
	for desttype := range m {
		s.nackByDest(m[desttype], desttype)
	}
}

func (s *MessageStore) doPermanentError(permerrors []queue.UidDest) {
	m := sortAck(permerrors)
	for desttype := range m {
		s.permErrorByDest(m[desttype], desttype)
	}
}

func (s *MessageStore) ackByDest(uids []utils.MyULID, dtype conf.DestinationType) {
	if len(uids) == 0 {
		return
	}

	sentDB := s.backend.GetPartition(Sent, dtype)
	messagesDB := s.backend.GetPartition(Messages, dtype)

	txn := s.badger.NewTransaction(true)

	defer txn.Discard()

	err := sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		return
	}
	err = messagesDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing message content from DB", "error", err)
		return
	}
	err = txn.Commit(nil)
	if err == nil {
		badgerGauge.WithLabelValues("sent", conf.DestinationNames[dtype]).Sub(float64(len(uids)))
		badgerGauge.WithLabelValues("messages", conf.DestinationNames[dtype]).Sub(float64(len(uids)))
	} else if err == badger.ErrConflict {
		// retry
		s.ackByDest(uids, dtype)
	} else {
		s.logger.Warn("Error commiting ACKs", "error", err)
	}
}

func (s *MessageStore) NACK(uid utils.MyULID, dest conf.DestinationType) {
	ackCounter.WithLabelValues("nack", conf.DestinationNames[dest]).Inc()
	_ = s.nackQueue.Put(uid, dest)
}

func (s *MessageStore) nackByDest(uids []utils.MyULID, dest conf.DestinationType) {
	if len(uids) == 0 {
		return
	}

	failedDB := s.backend.GetPartition(Failed, dest)
	sentDB := s.backend.GetPartition(Sent, dest)

	txn := s.badger.NewTransaction(true)
	defer txn.Discard()

	times := []byte(time.Now().Format(time.RFC3339))
	err := failedDB.AddManySame(uids, times, txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the Failed DB", "error", err)
	}
	err = sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		return
	}
	err = txn.Commit(nil)
	if err == nil {
		badgerGauge.WithLabelValues("sent", conf.DestinationNames[dest]).Sub(float64(len(uids)))
		badgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Add(float64(len(uids)))
	} else if err == badger.ErrConflict {
		// retry
		s.nackByDest(uids, dest)
	} else {
		s.logger.Warn("Error commiting NACKs", "error", err)
	}
}

func (s *MessageStore) PermError(uid utils.MyULID, dest conf.DestinationType) {
	ackCounter.WithLabelValues("permerror", conf.DestinationNames[dest]).Inc()
	_ = s.permerrorsQueue.Put(uid, dest)
}

func (s *MessageStore) permErrorByDest(uids []utils.MyULID, dest conf.DestinationType) {
	if len(uids) == 0 {
		return
	}
	sentDB := s.backend.GetPartition(Sent, dest)
	permDB := s.backend.GetPartition(PermErrors, dest)
	txn := s.badger.NewTransaction(true)

	defer txn.Discard()
	times := []byte(time.Now().Format(time.RFC3339))

	err := permDB.AddManySame(uids, times, txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the PermErrors DB", "error", err)
		return
	}
	err = sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		return
	}
	err = txn.Commit(nil)
	if err == nil {
		badgerGauge.WithLabelValues("permerrors", conf.DestinationNames[dest]).Add(float64(len(uids)))
		badgerGauge.WithLabelValues("sent", conf.DestinationNames[dest]).Sub(float64(len(uids)))
	} else if err == badger.ErrConflict {
		// retry
		s.permErrorByDest(uids, dest)
	} else {
		s.logger.Warn("Error commiting PermErrors", "error", err)
	}
}
