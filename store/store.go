package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awnumar/memguard"
	"github.com/dgraph-io/badger"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/db"
	"github.com/stephane-martin/skewer/utils/queue"
)

type Destinations struct {
	d uint64
}

func (dests *Destinations) Store(ds conf.DestinationType) {
	atomic.StoreUint64(&dests.d, uint64(ds))
}

func (dests *Destinations) Load() (res []conf.DestinationType) {
	ds := atomic.LoadUint64(&dests.d)
	if ds == 0 {
		return []conf.DestinationType{conf.Stderr}
	}
	res = make([]conf.DestinationType, 0, len(conf.Destinations))
	for _, dtype := range conf.Destinations {
		if ds&uint64(dtype) != 0 {
			res = append(res, dtype)
		}
	}
	return res
}

type storeMetrics struct {
	BadgerGauge *prometheus.GaugeVec
	AckCounter  *prometheus.CounterVec
}

func NewStoreMetrics() *storeMetrics {
	m := storeMetrics{}
	m.BadgerGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "skw_store_entries_gauge",
			Help: "number of messages stored in the badger database",
		},
		[]string{"queue", "destination"},
	)
	m.AckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_store_acks_total",
			Help: "number of ACKs received by the store",
		},
		[]string{"status", "destination"},
	)
	return &m
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

	metrics  *storeMetrics
	registry *prometheus.Registry

	//readyMutexes    map[conf.DestinationType](*sync.Mutex)
	//availConditions map[conf.DestinationType](*sync.Cond)

	wg        *sync.WaitGroup
	dests     *Destinations
	batchSize uint32

	ticker *time.Ticker
	logger log15.Logger

	closedChan     chan struct{}
	FatalErrorChan chan struct{}

	toStashQueue    *queue.MessageQueue
	ackQueue        *queue.AckQueue
	nackQueue       *queue.AckQueue
	permerrorsQueue *queue.AckQueue

	OutputsChans map[conf.DestinationType](chan *model.FullMessage)
	pool         *sync.Pool
}

func (s *MessageStore) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *MessageStore) Outputs(dest conf.DestinationType) chan *model.FullMessage {
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
	for queue.WaitManyAckQueues(s.ackQueue, s.nackQueue, s.permerrorsQueue) {
		s.doACK(s.ackQueue.GetMany(ackBatchSize))
		s.doNACK(s.nackQueue.GetMany(nackBatchSize))
		s.doPermanentError(s.permerrorsQueue.GetMany(nackBatchSize))
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
	for s.toStashQueue.Wait(0) {
		_, err = s.ingest(s.toStashQueue.GetMany(s.batchSize))
		if err != nil {
			s.logger.Warn("Ingestion error", "error", err)
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

	var messages map[ulid.ULID]*model.FullMessage
	for {
	wait_messages:
		for {
			select {
			case <-doneChan:
				return
			default:
				messages = s.retrieve(s.batchSize, d)
				if len(messages) > 0 {
					//store.logger.Debug("Messages to be sent to destination", "dest", d, "nb", len(messages))
					break wait_messages
				} else {
					time.Sleep(100 * time.Millisecond)
				}
			}
		}

		wg.Wait() // ensure at most one outputMsgs is running
		s.wg.Add(1)
		wg.Add(1)
		go func(msgs map[ulid.ULID]*model.FullMessage) {
			s.outputMsgs(doneChan, msgs, d)
			s.wg.Done()
			wg.Done()
		}(messages)
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

	for _, dest := range conf.Destinations {
		s.wg.Add(1)
		go s.forward(ctx, dest)
	}
}

func NewStore(ctx context.Context, cfg conf.StoreConfig, r kring.Ring, dests conf.DestinationType, l log15.Logger) (*MessageStore, error) {
	badgerOpts := badger.DefaultOptions
	badgerOpts.Dir = cfg.Dirname
	badgerOpts.ValueDir = cfg.Dirname
	badgerOpts.MaxTableSize = cfg.Maxsize
	badgerOpts.SyncWrites = cfg.FSync

	err := os.MkdirAll(cfg.Dirname, 0700)
	if err != nil {
		return nil, err
	}

	store := &MessageStore{metrics: NewStoreMetrics(), registry: prometheus.NewRegistry()}
	store.registry.MustRegister(store.metrics.BadgerGauge, store.metrics.AckCounter)
	store.logger = l.New("class", "MessageStore")
	store.dests = &Destinations{}
	store.dests.Store(dests)
	store.batchSize = cfg.BatchSize

	store.pool = &sync.Pool{
		New: func() interface{} {
			return &model.FullMessage{}
		},
	}

	store.toStashQueue = queue.NewMessageQueue()
	store.ackQueue = queue.NewAckQueue()
	store.nackQueue = queue.NewAckQueue()
	store.permerrorsQueue = queue.NewAckQueue()

	store.wg = &sync.WaitGroup{}

	store.closedChan = make(chan struct{})

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

	store.OutputsChans = map[conf.DestinationType](chan *model.FullMessage){}
	for _, dest := range conf.Destinations {
		store.OutputsChans[dest] = make(chan *model.FullMessage)
	}

	store.wg.Add(1)
	go store.init(ctx)

	return store, nil
}

func (s *MessageStore) outputMsgs(doneChan <-chan struct{}, messages map[ulid.ULID]*model.FullMessage, dest conf.DestinationType) {
	if len(messages) == 0 {
		s.logger.Debug("WOT?! 0 message were given for output", "dest", dest, "nb")
		return
	}
	output := s.Outputs(dest)
	//s.logger.Debug("ABOUT to send messages to output channel", "dest", dest, "nb", len(messages))
	for _, msg := range messages {
		select {
		case output <- msg:
		case <-doneChan:
			return
		}
	}
	//s.logger.Debug("DONE sending messages to output", "dest", dest)
}

func (s *MessageStore) WaitFinished() {
	<-s.closedChan
}

func (s *MessageStore) StoreAllSyslogConfigs(c conf.BaseConfig) (err error) {
	funcs := []utils.Func{}

	for _, c := range c.TcpSource {
		tcpConf := c
		funcs = append(funcs, func() error {
			return s.StoreSyslogConfig(tcpConf.ConfID, tcpConf.FilterSubConfig)
		})
	}

	for _, c := range c.UdpSource {
		udpConf := c
		funcs = append(funcs, func() error {
			return s.StoreSyslogConfig(udpConf.ConfID, udpConf.FilterSubConfig)
		})
	}

	for _, c := range c.RelpSource {
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

	funcs = append(funcs, func() error {
		return s.StoreSyslogConfig(c.Journald.ConfID, c.Journald.FilterSubConfig)
	})

	funcs = append(funcs, func() error {
		return s.StoreSyslogConfig(c.Accounting.ConfID, c.Accounting.FilterSubConfig)
	})

	return utils.Chain(funcs...)
}

func (s *MessageStore) StoreSyslogConfig(confID ulid.ULID, config conf.FilterSubConfig) error {
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
		s.metrics.BadgerGauge.WithLabelValues("syslogconf", "").Inc()
	}
	return nil
}

func (s *MessageStore) GetSyslogConfig(confID ulid.ULID) (*conf.FilterSubConfig, error) {
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
	s.metrics.BadgerGauge.WithLabelValues("syslogconf", "").Set(float64(s.syslogConfigsDB.Count(nil)))
	return s.badger.View(func(txn *badger.Txn) error {
		for dname, dtype := range conf.Destinations {
			s.metrics.BadgerGauge.WithLabelValues("messages", dname).Set(float64(s.backend.GetPartition(Messages, dtype).Count(txn)))
			s.metrics.BadgerGauge.WithLabelValues("ready", dname).Set(float64(s.backend.GetPartition(Ready, dtype).Count(txn)))
			s.metrics.BadgerGauge.WithLabelValues("sent", dname).Set(float64(s.backend.GetPartition(Sent, dtype).Count(txn)))
			s.metrics.BadgerGauge.WithLabelValues("failed", dname).Set(float64(s.backend.GetPartition(Failed, dtype).Count(txn)))
			s.metrics.BadgerGauge.WithLabelValues("permerrors", dname).Set(float64(s.backend.GetPartition(PermErrors, dtype).Count(txn)))
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
	orphaned_uids := []ulid.ULID{}
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
		orphaned_uids = append(orphaned_uids, uid)
	}

	// if no match, delete the message
	for _, uid := range orphaned_uids {
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
		uids := []ulid.ULID{}
		invalidUids := []ulid.ULID{}

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
					s.metrics.BadgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Sub(float64(len(invalidUids)))
				}
				return err
			}
			txn.Discard()
			return nil
		}

		//lok.Lock()
		readyBatch := map[ulid.ULID][]byte{}
		for _, uid := range uids {
			readyBatch[uid] = []byte("true")
		}
		err = readyDB.AddMany(readyBatch, txn)
		if err != nil {
			//lok.Unlock()
			s.logger.Warn("Error pushing entries from failed queue to ready queue", "error", err)
			txn.Discard()
			return err
		}

		err = failedDB.DeleteMany(uids, txn)
		if err != nil {
			//lok.Unlock()
			s.logger.Warn("Error deleting entries from failed queue", "error", err)
			txn.Discard()
			return err
		}

		err = txn.Commit(nil)
		if err == nil {
			//cond.Signal()
			//lok.Unlock()
			s.metrics.BadgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Sub(float64(len(invalidUids)))
			s.metrics.BadgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Add(float64(len(uids)))
			s.metrics.BadgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Sub(float64(len(uids)))
		} else {
			s.logger.Warn("Error commiting resetFailures", "error", err)
			//lok.Unlock()
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

func (s *MessageStore) Stash(m model.FullMessage) (fatal error, nonfatal error) {
	fatal = s.toStashQueue.Put(m)
	return fatal, nil
}

func (s *MessageStore) ingestByDest(queue map[ulid.ULID]([]byte), dest conf.DestinationType, txn *badger.Txn) (err error) {
	messagesDB := s.backend.GetPartition(Messages, dest)
	readyDB := s.backend.GetPartition(Ready, dest)

	err = messagesDB.AddMany(queue, txn)
	if err != nil {
		return err
	}

	readyQueue := map[ulid.ULID]([]byte){}
	for k := range queue {
		readyQueue[k] = []byte("true")
	}

	return readyDB.AddMany(readyQueue, txn)
}

func (s *MessageStore) ingest(queue []*model.FullMessage) (n int, err error) {
	if len(queue) == 0 {
		return 0, nil
	}
	var b []byte
	var m *model.FullMessage

	marshalledQueue := map[ulid.ULID][]byte{}
	for _, m = range queue {
		b, err = m.MarshalMsg(nil)
		if err == nil {
			if len(b) == 0 {
				s.logger.Warn("Ingestion of empty message", "uid", m.Uid)
			} else {
				marshalledQueue[m.Uid] = b
			}
		} else {
			s.logger.Warn("Discarded a message that could not be marshaled", "error", err)
		}
	}

	if len(marshalledQueue) == 0 {
		return 0, nil
	}

	dests := s.Destinations()

	/*
		for _, dest := range dests {
			s.readyMutexes[dest].Lock()
		}

		unlock := func() {
			for i := len(dests) - 1; i >= 0; i-- {
				s.readyMutexes[dests[i]].Unlock()
			}
		}
	*/

	txn := s.badger.NewTransaction(true)
	defer txn.Discard()

	for _, dest := range dests {
		err = s.ingestByDest(marshalledQueue, dest, txn)
		if err != nil {
			//unlock()
			return 0, err
		}
	}

	err = txn.Commit(nil)
	if err == nil {
		for _, dest := range dests {
			//s.availConditions[dest].Signal()
			s.metrics.BadgerGauge.WithLabelValues("messages", conf.DestinationNames[dest]).Add(float64(len(marshalledQueue)))
			s.metrics.BadgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Add(float64(len(marshalledQueue)))
		}
		//unlock()

		return len(marshalledQueue), nil
	} else if err == badger.ErrConflict {
		//unlock()
		return s.ingest(queue)
	} else {
		//unlock()
		return 0, err
	}

}

func (s *MessageStore) ReleaseMsg(msg *model.FullMessage) {
	s.pool.Put(msg)
}

func (s *MessageStore) retrieve(n uint32, dest conf.DestinationType) (messages map[ulid.ULID]*model.FullMessage) {
	txn := s.badger.NewTransaction(true)
	defer txn.Discard()

	readyDB := s.backend.GetPartition(Ready, dest)
	messagesDB := s.backend.GetPartition(Messages, dest)
	sentDB := s.backend.GetPartition(Sent, dest)
	messages = map[ulid.ULID]*model.FullMessage{}

	iter := readyDB.KeyIterator(n, txn)
	var fetched uint32
	invalidEntries := []ulid.ULID{}
	var message *model.FullMessage

	for iter.Rewind(); iter.Valid() && fetched < n; iter.Next() {
		uid := iter.Key()
		message_b, err := messagesDB.Get(uid, txn)
		if err == nil {
			if len(message_b) > 0 {
				message = s.pool.Get().(*model.FullMessage)
				_, err := message.UnmarshalMsg(message_b)
				if err == nil {
					messages[uid] = message
					fetched++
				} else {
					invalidEntries = append(invalidEntries, uid)
					s.logger.Warn("retrieved invalid entry", "uid", uid, "message", string(message_b), "dest", dest, "error", err)
				}
			} else {
				invalidEntries = append(invalidEntries, uid)
				s.logger.Warn("retrieved empty entry", "uid", uid)
			}
		} else {
			s.logger.Warn("Error getting message content from message queue", "uid", uid, "dest", dest, "error", err)
			iter.Close()
			return map[ulid.ULID]*model.FullMessage{}
		}
	}
	iter.Close()

	if len(invalidEntries) > 0 {
		s.logger.Info("Found invalid entries", "number", len(invalidEntries))
		err := readyDB.DeleteMany(invalidEntries, txn)
		if err != nil {
			s.logger.Warn("Error deleting invalid entries from 'ready' queue", "error", err)
			return map[ulid.ULID]*model.FullMessage{}
		}
		err = messagesDB.DeleteMany(invalidEntries, txn)
		if err != nil {
			s.logger.Warn("Error deleting invalid entries from 'messages' queue", "error", err)
			return map[ulid.ULID]*model.FullMessage{}
		}
	}

	if len(messages) == 0 {
		if len(invalidEntries) > 0 {
			err := txn.Commit(nil)
			if err == nil {
				s.metrics.BadgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Sub(float64(len(invalidEntries)))
				s.metrics.BadgerGauge.WithLabelValues("messages", conf.DestinationNames[dest]).Sub(float64(len(invalidEntries)))
			}
		}
		return map[ulid.ULID]*model.FullMessage{}
	}

	sentBatch := map[ulid.ULID][]byte{}
	for uid := range messages {
		sentBatch[uid] = []byte("true")
	}
	err := sentDB.AddMany(sentBatch, txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the 'sent' queue", "error", err)
		return map[ulid.ULID]*model.FullMessage{}
	}
	readyBatch := make([]ulid.ULID, 0, len(sentBatch))
	for k := range sentBatch {
		readyBatch = append(readyBatch, k)
	}
	err = readyDB.DeleteMany(readyBatch, txn)
	if err != nil {
		s.logger.Warn("Error deleting messages from the 'ready' queue", "error", err)
		return map[ulid.ULID]*model.FullMessage{}
	}

	err = txn.Commit(nil)
	if err == nil {
		s.metrics.BadgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Sub(float64(len(invalidEntries)))
		s.metrics.BadgerGauge.WithLabelValues("messages", conf.DestinationNames[dest]).Sub(float64(len(invalidEntries)))
		s.metrics.BadgerGauge.WithLabelValues("sent", conf.DestinationNames[dest]).Add(float64(len(sentBatch)))
		s.metrics.BadgerGauge.WithLabelValues("ready", conf.DestinationNames[dest]).Sub(float64(len(readyBatch)))
		return messages
	} else if err == badger.ErrConflict {
		// retry
		return s.retrieve(n, dest)
	} else {
		s.logger.Warn("Error committing to badger in retrieve", "error", err)
		return map[ulid.ULID]*model.FullMessage{}
	}

}

func sortAck(acks []queue.UidDest) (res map[conf.DestinationType]([]ulid.ULID)) {
	var ok bool
	res = map[conf.DestinationType]([]ulid.ULID){}
	for _, ack := range acks {
		if _, ok = res[ack.Dest]; !ok {
			res[ack.Dest] = make([]ulid.ULID, 0, len(acks))
		}
		res[ack.Dest] = append(res[ack.Dest], ack.Uid)
	}
	return res
}

func (s *MessageStore) ACK(uid ulid.ULID, dest conf.DestinationType) {
	s.metrics.AckCounter.WithLabelValues("ack", conf.DestinationNames[dest]).Inc()
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

func (s *MessageStore) ackByDest(uids []ulid.ULID, dtype conf.DestinationType) {
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
	uids_map := map[ulid.ULID]bool{}
	for _, uid := range uids {
		uids_map[uid] = true
	}
	uids = make([]ulid.ULID, 0, len(uids_map))
	for uid := range uids_map {
		uids = append(uids, uid)
	}
	err = messagesDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing message content from DB", "error", err)
		return
	}
	err = txn.Commit(nil)
	if err == nil {
		s.metrics.BadgerGauge.WithLabelValues("sent", conf.DestinationNames[dtype]).Sub(float64(len(uids)))
		s.metrics.BadgerGauge.WithLabelValues("messages", conf.DestinationNames[dtype]).Sub(float64(len(uids)))
	} else if err == badger.ErrConflict {
		// retry
		s.ackByDest(uids, dtype)
	} else {
		s.logger.Warn("Error commiting ACKs", "error", err)
	}
}

func (s *MessageStore) NACK(uid ulid.ULID, dest conf.DestinationType) {
	s.metrics.AckCounter.WithLabelValues("nack", conf.DestinationNames[dest]).Inc()
	_ = s.nackQueue.Put(uid, dest)
}

func (s *MessageStore) nackByDest(uids []ulid.ULID, dest conf.DestinationType) {
	if len(uids) == 0 {
		return
	}

	failedDB := s.backend.GetPartition(Failed, dest)
	sentDB := s.backend.GetPartition(Sent, dest)

	txn := s.badger.NewTransaction(true)

	defer txn.Discard()

	times := time.Now().Format(time.RFC3339)
	failedBatch := map[ulid.ULID][]byte{}
	for _, uid := range uids {
		failedBatch[uid] = []byte(times)
	}
	err := failedDB.AddMany(failedBatch, txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the Failed DB", "error", err)
	}
	uids = make([]ulid.ULID, 0, len(failedBatch))
	for uid := range failedBatch {
		uids = append(uids, uid)
	}
	err = sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		return
	}
	err = txn.Commit(nil)
	if err == nil {
		s.metrics.BadgerGauge.WithLabelValues("sent", conf.DestinationNames[dest]).Sub(float64(len(uids)))
		s.metrics.BadgerGauge.WithLabelValues("failed", conf.DestinationNames[dest]).Add(float64(len(failedBatch)))
	} else if err == badger.ErrConflict {
		// retry
		s.nackByDest(uids, dest)
	} else {
		s.logger.Warn("Error commiting NACKs", "error", err)
	}
}

func (s *MessageStore) PermError(uid ulid.ULID, dest conf.DestinationType) {
	s.metrics.AckCounter.WithLabelValues("permerror", conf.DestinationNames[dest]).Inc()
	_ = s.permerrorsQueue.Put(uid, dest)
}

func (s *MessageStore) permErrorByDest(uids []ulid.ULID, dest conf.DestinationType) {
	if len(uids) == 0 {
		return
	}
	sentDB := s.backend.GetPartition(Sent, dest)
	permDB := s.backend.GetPartition(PermErrors, dest)
	times := time.Now().Format(time.RFC3339)
	permBatch := map[ulid.ULID][]byte{}
	for _, uid := range uids {
		permBatch[uid] = []byte(times)
	}
	txn := s.badger.NewTransaction(true)
	defer txn.Discard()
	err := permDB.AddMany(permBatch, txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the PermErrors DB", "error", err)
		return
	}
	uids = make([]ulid.ULID, 0, len(permBatch))
	for uid := range permBatch {
		uids = append(uids, uid)
	}
	err = sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		return
	}
	err = txn.Commit(nil)
	if err == nil {
		s.metrics.BadgerGauge.WithLabelValues("permerrors", conf.DestinationNames[dest]).Add(float64(len(permBatch)))
		s.metrics.BadgerGauge.WithLabelValues("sent", conf.DestinationNames[dest]).Sub(float64(len(uids)))
	} else if err == badger.ErrConflict {
		// retry
		s.permErrorByDest(uids, dest)
	} else {
		s.logger.Warn("Error commiting PermErrors", "error", err)
	}
}
