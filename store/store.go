package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils/db"
	"github.com/stephane-martin/skewer/utils/queue"
)

type storeMetrics struct {
	BadgerGauge *prometheus.GaugeVec
}

func NewStoreMetrics() *storeMetrics {
	m := &storeMetrics{}
	m.BadgerGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "skw_badger_entries_gauge",
			Help: "number of messages stored in the badger database",
		},
		[]string{"partition"},
	)
	return m
}

type MessageStore struct {
	badger          *badger.DB
	messagesDB      db.Partition
	readyDB         db.Partition
	sentDB          db.Partition
	failedDB        db.Partition
	permerrorsDB    db.Partition
	syslogConfigsDB db.Partition

	metrics  *storeMetrics
	registry *prometheus.Registry

	readyMu      *sync.Mutex
	availMsgCond *sync.Cond
	failedMu     *sync.Mutex
	msgsMu       *sync.Mutex

	wg *sync.WaitGroup

	ticker *time.Ticker
	logger log15.Logger

	closedChan     chan struct{}
	FatalErrorChan chan struct{}

	toStashQueue    *queue.MessageQueue
	ackQueue        *queue.AckQueue
	nackQueue       *queue.AckQueue
	permerrorsQueue *queue.AckQueue

	OutputsChan chan *model.FullMessage
	pool        *sync.Pool
}

func (s *MessageStore) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *MessageStore) Outputs() chan *model.FullMessage {
	return s.OutputsChan
}

func (s *MessageStore) Errors() chan struct{} {
	return s.FatalErrorChan
}

func NewStore(ctx context.Context, cfg conf.StoreConfig, l log15.Logger) (Store, error) {

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
	store.registry.MustRegister(store.metrics.BadgerGauge)
	store.logger = l.New("class", "MessageStore")

	store.pool = &sync.Pool{
		New: func() interface{} {
			return &model.FullMessage{}
		},
	}

	store.toStashQueue = queue.NewMessageQueue()
	store.ackQueue = queue.NewAckQueue()
	store.nackQueue = queue.NewAckQueue()
	store.permerrorsQueue = queue.NewAckQueue()

	store.readyMu = &sync.Mutex{}
	store.availMsgCond = sync.NewCond(store.readyMu)
	store.failedMu = &sync.Mutex{}
	store.msgsMu = &sync.Mutex{}
	store.wg = &sync.WaitGroup{}

	store.closedChan = make(chan struct{})

	kv, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, err
	}
	store.badger = kv

	store.messagesDB = db.NewPartition(kv, []byte("messages"))
	if len(cfg.Secret) > 0 {
		store.messagesDB = db.NewEncryptedPartition(store.messagesDB, cfg.SecretB)
		store.logger.Info("The badger store is encrypted")
	}

	store.readyDB = db.NewPartition(kv, []byte("ready"))
	store.sentDB = db.NewPartition(kv, []byte("sent"))
	store.failedDB = db.NewPartition(kv, []byte("failed"))
	store.permerrorsDB = db.NewPartition(kv, []byte("permerrors"))
	store.syslogConfigsDB = db.NewPartition(kv, []byte("configs"))

	// only once, push back messages from previous run that may have been stuck in the sent queue
	store.resetStuckInSent()

	// prune orphaned messages
	store.pruneOrphaned()

	// count existing messages in badger and report to metrics
	store.initGauge()

	store.FatalErrorChan = make(chan struct{})
	store.ticker = time.NewTicker(time.Minute)

	store.wg.Add(1)
	go func() {
		for queue.WaitManyAckQueues(store.ackQueue, store.nackQueue, store.permerrorsQueue) {
			store.doACK(store.ackQueue.GetMany(300))
			store.doNACK(store.nackQueue.GetMany(300))
			store.doPermanentError(store.permerrorsQueue.GetMany(300))
		}
		//store.logger.Debug("Store goroutine WaitAck ended")
		store.wg.Done()
	}()

	store.wg.Add(1)
	go func() {
		<-ctx.Done()
		//store.logger.Debug("Store is asked to stop")
		store.toStashQueue.Dispose()
		store.ackQueue.Dispose()
		store.nackQueue.Dispose()
		store.permerrorsQueue.Dispose()
		store.wg.Done()
	}()

	store.wg.Add(1)
	go func() {
		var err error
		for store.toStashQueue.Wait(0) {
			_, err = store.ingest(store.toStashQueue.GetMany(1000))
			if err != nil {
				store.logger.Warn("Ingestion error in the Store", "error", err)
			}
		}
		// store.logger.Debug("Store goroutine WaitMessages ended")
		store.wg.Done()
	}()

	store.wg.Add(1)
	go func() {
		defer store.wg.Done()
		for {
			select {
			/*
				if err == badger.ErrNoRoom {
					TODO: check that in another place
					store.logger.Crit("The store is full!")
					close(store.FatalErrorChan) // signal the caller service than we should stop everything
				} else {
					store.logger.Warn("Store unexpected error", "error", err)
				}
			*/

			case <-store.ticker.C:
				store.resetFailures()
			case <-ctx.Done():
				store.ticker.Stop()
				// store.logger.Debug("Store ticker has been stopped")
				return
			}
		}
	}()

	store.OutputsChan = make(chan *model.FullMessage)

	store.wg.Add(1)
	go func() {
		doneChan := ctx.Done()
		store.readyMu.Lock()
		defer func() {
			store.readyMu.Unlock()
			close(store.OutputsChan)
			store.wg.Done()
		}()
		var messages map[ulid.ULID]*model.FullMessage
		for {
		wait_messages:
			for {
				select {
				case <-doneChan:
					return
				default:
					messages = store.retrieve(1000)
					if len(messages) > 0 {
						break wait_messages
					} else {
						store.availMsgCond.Wait()
					}
				}
			}
			store.outputMsgs(doneChan, messages)

		}
	}()

	go func() {
		<-ctx.Done()
		store.availMsgCond.Signal()
	}()

	go func() {
		store.wg.Wait()
		store.closeBadgers()
		close(store.closedChan)
	}()

	return store, nil
}

func (s *MessageStore) outputMsgs(doneChan <-chan struct{}, messages map[ulid.ULID]*model.FullMessage) {
	if len(messages) == 0 {
		return
	}
	s.readyMu.Unlock()
	defer s.readyMu.Lock()
	for _, msg := range messages {
		select {
		case s.OutputsChan <- msg:
		case <-doneChan:
			return
		}
	}
}

func (s *MessageStore) WaitFinished() {
	<-s.closedChan
}

func (s *MessageStore) StoreAllSyslogConfigs(c conf.BaseConfig) (err error) {
	for _, sysconf := range c.Syslog {
		err = s.StoreSyslogConfig(sysconf.ConfID, sysconf.FilterSubConfig)
		if err != nil {
			return err
		}
	}

	err = s.StoreSyslogConfig(c.Journald.ConfID, c.Journald.FilterSubConfig)
	if err != nil {
		return err
	}

	return s.StoreSyslogConfig(c.Accounting.ConfID, c.Accounting.FilterSubConfig)
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
		s.metrics.BadgerGauge.WithLabelValues("syslogconf").Inc()
	}
	return nil
}

func (s *MessageStore) GetSyslogConfig(confID ulid.ULID) (*conf.SyslogConfig, error) {
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

func (s *MessageStore) initGauge() {
	if s.metrics != nil {
		s.metrics.BadgerGauge.WithLabelValues("messages").Set(float64(s.messagesDB.Count(nil)))
		s.metrics.BadgerGauge.WithLabelValues("ready").Set(float64(s.readyDB.Count(nil)))
		s.metrics.BadgerGauge.WithLabelValues("sent").Set(float64(s.sentDB.Count(nil)))
		s.metrics.BadgerGauge.WithLabelValues("failed").Set(float64(s.failedDB.Count(nil)))
		s.metrics.BadgerGauge.WithLabelValues("permerrors").Set(float64(s.permerrorsDB.Count(nil)))
		s.metrics.BadgerGauge.WithLabelValues("syslogconf").Set(float64(s.syslogConfigsDB.Count(nil)))
	}
}

func (s *MessageStore) closeBadgers() {
	err := s.badger.Close()
	if err != nil {
		s.logger.Warn("Error closing the badger", "error", err)
	}
	s.logger.Debug("Badger databases are closed")
}

func (s *MessageStore) pruneOrphaned() {
	// find if we have some old full messages
	var err error
	var have bool
	txn := s.badger.NewTransaction(true)
	defer txn.Discard()
	uids := s.messagesDB.ListKeys(txn)

	// check if the corresponding uid exists in "ready" or "failed" or "permerrors"
	orphaned_uids := []ulid.ULID{}
	for _, uid := range uids {
		have, err = s.readyDB.Exists(uid, txn)
		if err != nil {
			return
		}
		if have {
			continue
		}
		have, err = s.failedDB.Exists(uid, txn)
		if err != nil {
			return
		}
		if have {
			continue
		}
		have, err = s.permerrorsDB.Exists(uid, txn)
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
		err = s.messagesDB.Delete(uid, txn)
		if err != nil {
			return
		}
	}
	txn.Commit(nil)
}

func (s *MessageStore) resetStuckInSent() {
	// push back to "Ready" the messages that were sent out of the Store in the
	// last execution of skewer, but never were ACKed or NACKed
	txn := s.badger.NewTransaction(true)
	defer txn.Discard()
	uids := s.sentDB.ListKeys(txn)
	s.logger.Debug("Pushing back stuck messages from Sent to Ready", "nb_messages", len(uids))
	err := s.sentDB.DeleteMany(uids, txn)
	if err != nil {
		return
	}
	for _, uid := range uids {
		err = s.readyDB.Set(uid, []byte("true"), txn)
		if err != nil {
			return
		}
	}
	txn.Commit(nil)
}

func (s *MessageStore) ReadAllBadgers() (map[string]string, map[string]string, map[string]string) {
	return nil, nil, nil // FIXME
}

func (s *MessageStore) resetFailures() {
	// push back messages from "failed" to "ready"
	s.failedMu.Lock()
	defer s.failedMu.Unlock()

	for {
		txn := s.badger.NewTransaction(true)
		now := time.Now()
		iter := s.failedDB.KeyValueIterator(1000, txn)
		uids := []ulid.ULID{}
		invalidUids := []ulid.ULID{}

		for iter.Rewind(); iter.Valid(); iter.Next() {
			uid := iter.Key()
			time_s := string(iter.Value())
			t, err := time.Parse(time.RFC3339, time_s)
			if err == nil {
				if now.Sub(t) >= time.Minute {
					// messages that failed to be delivered to Kafka should be tried again after 1 minute
					uids = append(uids, uid)
				}
			} else {
				invalidUids = append(invalidUids, uid)
			}
		}
		iter.Close()

		if len(invalidUids) > 0 {
			s.logger.Info("Found invalid entries in 'failed'", "number", len(invalidUids))
			err := s.failedDB.DeleteMany(invalidUids, txn)
			if err != nil {
				s.logger.Warn("Error deleting invalid entries", "error", err)
				txn.Discard()
				return
			}
		}

		if len(uids) == 0 {
			if txn.Commit(nil) == nil {
				s.metrics.BadgerGauge.WithLabelValues("failed").Sub(float64(len(invalidUids)))
			}
			return
		}

		s.readyMu.Lock()
		readyBatch := map[ulid.ULID][]byte{}
		for _, uid := range uids {
			readyBatch[uid] = []byte("true")
		}
		err := s.readyDB.AddMany(readyBatch, txn)
		if err != nil {
			s.logger.Warn("Error pushing entries from failed queue to ready queue", "error", err)
			s.readyMu.Unlock()
			txn.Discard()
			return
		}

		err = s.failedDB.DeleteMany(uids, txn)
		if err != nil {
			s.logger.Warn("Error deleting entries from failed queue", "error", err)
			s.readyMu.Unlock()
			txn.Discard()
			return
		}

		s.readyMu.Unlock()

		err = txn.Commit(nil)
		if err == nil {
			s.availMsgCond.Signal()
			s.metrics.BadgerGauge.WithLabelValues("failed").Sub(float64(len(invalidUids)))
			s.metrics.BadgerGauge.WithLabelValues("ready").Add(float64(len(uids)))
			s.metrics.BadgerGauge.WithLabelValues("failed").Sub(float64(len(uids)))
		} else {
			s.logger.Warn("Error commiting resetFailures", "error", err)
		}
	}

	err := s.badger.PurgeOlderVersions()
	if err == nil {
		err = s.badger.RunValueLogGC(0.5)
		if err != nil {
			s.logger.Warn("Error garbage collecting badger", "error", err)
		}
	} else {
		s.logger.Warn("Error purging badger", "error", err)
	}

}

func (s *MessageStore) Stash(m model.FullMessage) (fatal error, nonfatal error) {
	s.toStashQueue.Put(m)
	return nil, nil
}

func (s *MessageStore) ingest(queue []*model.FullMessage) (int, error) {
	if len(queue) == 0 {
		return 0, nil
	}

	marshalledQueue := map[ulid.ULID][]byte{}
	for _, m := range queue {
		b, err := m.MarshalMsg(nil)
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

	s.readyMu.Lock()
	s.msgsMu.Lock()
	txn := s.badger.NewTransaction(true)

	defer func() {
		txn.Discard()
	}()

	err := s.messagesDB.AddMany(marshalledQueue, txn)
	if err != nil {
		s.msgsMu.Unlock()
		s.readyMu.Unlock()
		s.logger.Warn("Error adding ingested message to the messages queue", "error", err)
		return 0, err
	}

	for k := range marshalledQueue {
		marshalledQueue[k] = []byte("true")
	}

	err = s.readyDB.AddMany(marshalledQueue, txn)
	if err != nil {
		s.msgsMu.Unlock()
		s.readyMu.Unlock()
		s.logger.Warn("Error adding ingested message to the ready queue", "error", err)
		return 0, err
	}

	err = txn.Commit(nil)
	s.msgsMu.Unlock()
	s.readyMu.Unlock()

	if err == nil {
		s.availMsgCond.Signal()
		s.metrics.BadgerGauge.WithLabelValues("messages").Add(float64(len(marshalledQueue)))
		s.metrics.BadgerGauge.WithLabelValues("ready").Add(float64(len(marshalledQueue)))
		return len(marshalledQueue), nil
	} else if err == badger.ErrConflict {
		// retry
		return s.ingest(queue)
	} else {
		return 0, err
	}

}

func (s *MessageStore) ReleaseMsg(msg *model.FullMessage) {
	s.pool.Put(msg)
}

func (s *MessageStore) retrieve(n int) (messages map[ulid.ULID]*model.FullMessage) {
	s.msgsMu.Lock()
	txn := s.badger.NewTransaction(true)
	defer func() {
		txn.Discard()
	}()

	messages = map[ulid.ULID]*model.FullMessage{}

	iter := s.readyDB.KeyIterator(n, txn)
	var fetched int = 0
	invalidEntries := []ulid.ULID{}
	var message *model.FullMessage

	for iter.Rewind(); iter.Valid() && fetched < n; iter.Next() {
		uid := iter.Key()
		message_b, err := s.messagesDB.Get(uid, txn)
		if err == nil {
			if len(message_b) > 0 {
				message = s.pool.Get().(*model.FullMessage)
				_, err := message.UnmarshalMsg(message_b)
				if err == nil {
					messages[uid] = message
					fetched++
				} else {
					invalidEntries = append(invalidEntries, uid)
					s.logger.Debug("retrieved invalid entry", "uid", uid, "message", string(message_b), "error", err)
				}
			} else {
				invalidEntries = append(invalidEntries, uid)
				s.logger.Debug("retrieved empty entry", "uid", uid)
			}
		} else {
			s.logger.Warn("Error getting message content from message queue", "uid", uid, "error", err)
			iter.Close()
			s.msgsMu.Unlock()
			return map[ulid.ULID]*model.FullMessage{}
		}
	}
	iter.Close()

	if len(invalidEntries) > 0 {
		s.logger.Info("Found invalid entries", "number", len(invalidEntries))
		err := s.readyDB.DeleteMany(invalidEntries, txn)
		if err != nil {
			s.logger.Warn("Error deleting invalid entries from 'ready' queue", "error", err)
			s.msgsMu.Unlock()
			return map[ulid.ULID]*model.FullMessage{}
		}
		err = s.messagesDB.DeleteMany(invalidEntries, txn)
		if err != nil {
			s.logger.Warn("Error deleting invalid entries from 'messages' queue", "error", err)
			s.msgsMu.Unlock()
			return map[ulid.ULID]*model.FullMessage{}
		}
	}

	if len(messages) == 0 {
		err := txn.Commit(nil)
		s.msgsMu.Unlock()
		if err == nil {
			s.metrics.BadgerGauge.WithLabelValues("ready").Sub(float64(len(invalidEntries)))
			s.metrics.BadgerGauge.WithLabelValues("messages").Sub(float64(len(invalidEntries)))
		}
		return map[ulid.ULID]*model.FullMessage{}
	}

	sentBatch := map[ulid.ULID][]byte{}
	for uid := range messages {
		sentBatch[uid] = []byte("true")
	}
	err := s.sentDB.AddMany(sentBatch, txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the 'sent' queue", "error", err)
		s.msgsMu.Unlock()
		return map[ulid.ULID]*model.FullMessage{}
	}
	readyBatch := make([]ulid.ULID, 0, len(sentBatch))
	for k := range sentBatch {
		readyBatch = append(readyBatch, k)
	}
	err = s.readyDB.DeleteMany(readyBatch, txn)
	if err != nil {
		s.logger.Warn("Error deleting messages from the 'ready' queue", "error", err)
		s.msgsMu.Unlock()
		return map[ulid.ULID]*model.FullMessage{}
	}

	err = txn.Commit(nil)
	s.msgsMu.Unlock()
	if err == nil {
		s.metrics.BadgerGauge.WithLabelValues("ready").Sub(float64(len(invalidEntries)))
		s.metrics.BadgerGauge.WithLabelValues("messages").Sub(float64(len(invalidEntries)))
		s.metrics.BadgerGauge.WithLabelValues("sent").Add(float64(len(sentBatch)))
		s.metrics.BadgerGauge.WithLabelValues("ready").Sub(float64(len(readyBatch)))
		return messages
	} else if err == badger.ErrConflict {
		// retry
		return s.retrieve(n)
	} else {
		s.logger.Warn("Error committing to badger in retrieve", "error", err)
		return map[ulid.ULID]*model.FullMessage{}
	}

}

func (s *MessageStore) ACK(uid ulid.ULID) {
	s.ackQueue.Put(uid)
}

func (s *MessageStore) doACK(uids []ulid.ULID) {
	if len(uids) == 0 {
		return
	}

	s.msgsMu.Lock()
	txn := s.badger.NewTransaction(true)

	defer func() {
		txn.Discard()
	}()

	err := s.sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		s.msgsMu.Unlock()
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
	err = s.messagesDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing message content from DB", "error", err)
		s.msgsMu.Unlock()
		return
	}
	err = txn.Commit(nil)
	s.msgsMu.Unlock()
	if err == nil {
		s.metrics.BadgerGauge.WithLabelValues("sent").Sub(float64(len(uids)))
		s.metrics.BadgerGauge.WithLabelValues("messages").Sub(float64(len(uids)))
	} else if err == badger.ErrConflict {
		// retry
		s.doACK(uids)
	} else {
		s.logger.Warn("Error commiting ACKs", "error", err)
	}
}

func (s *MessageStore) NACK(uid ulid.ULID) {
	s.nackQueue.Put(uid)
}

func (s *MessageStore) doNACK(uids []ulid.ULID) {
	if len(uids) == 0 {
		return
	}

	s.failedMu.Lock()
	txn := s.badger.NewTransaction(true)

	defer func() {
		txn.Discard()
	}()

	times := time.Now().Format(time.RFC3339)
	failedBatch := map[ulid.ULID][]byte{}
	for _, uid := range uids {
		failedBatch[uid] = []byte(times)
	}
	err := s.failedDB.AddMany(failedBatch, txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the Failed DB", "error", err)
		s.failedMu.Unlock()
		return
	}
	uids = make([]ulid.ULID, 0, len(failedBatch))
	for uid := range failedBatch {
		uids = append(uids, uid)
	}
	err = s.sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		s.failedMu.Unlock()
		return
	}
	err = txn.Commit(nil)
	s.failedMu.Unlock()
	if err == nil {
		s.metrics.BadgerGauge.WithLabelValues("sent").Sub(float64(len(uids)))
		s.metrics.BadgerGauge.WithLabelValues("failed").Add(float64(len(failedBatch)))
	} else if err == badger.ErrConflict {
		// retry
		s.doNACK(uids)
	} else {
		s.logger.Warn("Error commiting NACKs", "error", err)
	}
}

func (s *MessageStore) PermError(uid ulid.ULID) {
	s.permerrorsQueue.Put(uid)
}

func (s *MessageStore) doPermanentError(uids []ulid.ULID) {
	if len(uids) == 0 {
		return
	}
	times := time.Now().Format(time.RFC3339)
	permBatch := map[ulid.ULID][]byte{}
	for _, uid := range uids {
		permBatch[uid] = []byte(times)
	}
	txn := s.badger.NewTransaction(true)
	defer txn.Discard()
	err := s.permerrorsDB.AddMany(permBatch, txn)
	if err != nil {
		s.logger.Warn("Error copying messages to the PermErrors DB", "error", err)
		return
	}
	uids = make([]ulid.ULID, 0, len(permBatch))
	for uid := range permBatch {
		uids = append(uids, uid)
	}
	err = s.sentDB.DeleteMany(uids, txn)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		return
	}
	err = txn.Commit(nil)
	if err == nil {
		s.metrics.BadgerGauge.WithLabelValues("permerrors").Add(float64(len(permBatch)))
		s.metrics.BadgerGauge.WithLabelValues("sent").Sub(float64(len(uids)))
	} else if err == badger.ErrConflict {
		// retry
		s.doPermanentError(uids)
	} else {
		s.logger.Warn("Error commiting PermErrors", "error", err)
	}

}
