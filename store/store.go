package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
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
	badger          *badger.KV
	messagesDB      utils.Partition
	readyDB         utils.Partition
	sentDB          utils.Partition
	failedDB        utils.Partition
	permerrorsDB    utils.Partition
	syslogConfigsDB utils.Partition

	metrics  *storeMetrics
	registry *prometheus.Registry

	ready_mu     *sync.Mutex
	availMsgCond *sync.Cond
	failed_mu    *sync.Mutex
	messages_mu  *sync.Mutex

	wg *sync.WaitGroup

	ticker *time.Ticker
	logger log15.Logger

	closedChan     chan struct{}
	FatalErrorChan chan struct{}

	toStashQueue    *queue.MessageQueue
	ackQueue        *queue.AckQueue
	nackQueue       *queue.AckQueue
	permerrorsQueue *queue.AckQueue

	OutputsChan chan *model.TcpUdpParsedMessage
}

func (s *MessageStore) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *MessageStore) Outputs() chan *model.TcpUdpParsedMessage {
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

	store.toStashQueue = queue.NewMessageQueue()
	store.ackQueue = queue.NewAckQueue()
	store.nackQueue = queue.NewAckQueue()
	store.permerrorsQueue = queue.NewAckQueue()

	store.ready_mu = &sync.Mutex{}
	store.availMsgCond = sync.NewCond(store.ready_mu)
	store.failed_mu = &sync.Mutex{}
	store.messages_mu = &sync.Mutex{}
	store.wg = &sync.WaitGroup{}

	store.closedChan = make(chan struct{})

	kv, err := badger.NewKV(&badgerOpts)
	if err != nil {
		return nil, err
	}
	store.badger = kv

	store.messagesDB = utils.NewPartition(kv, "messages")
	if len(cfg.Secret) > 0 {
		store.messagesDB = utils.NewEncryptedPartition(store.messagesDB, cfg.SecretB)
		store.logger.Info("The badger store is encrypted")
	}

	store.readyDB = utils.NewPartition(kv, "ready")
	store.sentDB = utils.NewPartition(kv, "sent")
	store.failedDB = utils.NewPartition(kv, "failed")
	store.permerrorsDB = utils.NewPartition(kv, "permerrors")
	store.syslogConfigsDB = utils.NewPartition(kv, "configs")

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
		for store.toStashQueue.Wait() {
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

	store.OutputsChan = make(chan *model.TcpUdpParsedMessage)

	store.wg.Add(1)
	go func() {
		doneChan := ctx.Done()
		store.ready_mu.Lock()
		defer func() {
			store.ready_mu.Unlock()
			close(store.OutputsChan)
			store.wg.Done()
		}()
		var messages map[string]*model.TcpUdpParsedMessage
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

			if len(messages) > 0 {
				store.ready_mu.Unlock()
				// loop on the available messages, but immediately stop if the context is canceled
				for _, msg := range messages {
					select {
					case store.OutputsChan <- msg:
					case <-doneChan:
						store.ready_mu.Lock()
						return
					}
				}
				store.ready_mu.Lock()
			}
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

func (s *MessageStore) WaitFinished() {
	<-s.closedChan
}

func (s *MessageStore) StoreAllSyslogConfigs(c conf.BaseConfig) (err error) {
	for _, sysconf := range c.Syslog {
		err = s.StoreSyslogConfig(sysconf)
		if err != nil {
			return err
		}
	}

	journalSyslogConf := conf.SyslogConfig{
		TopicTmpl:     c.Journald.TopicTmpl,
		TopicFunc:     c.Journald.TopicFunc,
		PartitionTmpl: c.Journald.PartitionTmpl,
		PartitionFunc: c.Journald.PartitionFunc,
		FilterFunc:    c.Journald.FilterFunc,
		ConfID:        c.Journald.ConfID,
	}
	err = s.StoreSyslogConfig(journalSyslogConf)
	if err != nil {
		return err
	}

	accSyslogConf := conf.SyslogConfig{
		TopicTmpl:     c.Accounting.TopicTmpl,
		TopicFunc:     c.Accounting.TopicFunc,
		PartitionTmpl: c.Accounting.PartitionTmpl,
		PartitionFunc: c.Accounting.PartitionFunc,
		FilterFunc:    c.Accounting.FilterFunc,
		ConfID:        c.Accounting.ConfID,
	}
	err = s.StoreSyslogConfig(accSyslogConf)
	if err != nil {
		return err
	}

	return nil
}

func (s *MessageStore) StoreSyslogConfig(config conf.SyslogConfig) error {
	data := config.Export()
	exists, err := s.syslogConfigsDB.Exists(config.ConfID)
	if err != nil {
		return err
	}
	if !exists {
		err = s.syslogConfigsDB.Set(config.ConfID, data)
		if err != nil {
			return err
		}
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("syslogconf").Inc()
		}
	}
	return nil
}

func (s *MessageStore) GetSyslogConfig(confID string) (*conf.SyslogConfig, error) {
	data, err := s.syslogConfigsDB.Get(confID)
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
		s.metrics.BadgerGauge.WithLabelValues("messages").Set(float64(s.messagesDB.Count()))
		s.metrics.BadgerGauge.WithLabelValues("ready").Set(float64(s.readyDB.Count()))
		s.metrics.BadgerGauge.WithLabelValues("sent").Set(float64(s.sentDB.Count()))
		s.metrics.BadgerGauge.WithLabelValues("failed").Set(float64(s.failedDB.Count()))
		s.metrics.BadgerGauge.WithLabelValues("permerrors").Set(float64(s.permerrorsDB.Count()))
		s.metrics.BadgerGauge.WithLabelValues("syslogconf").Set(float64(s.syslogConfigsDB.Count()))
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

	uids := s.messagesDB.ListKeys()

	// check if the corresponding uid exists in "ready" or "failed" or "permerrors"
	orphaned_uids := []string{}
	for _, uid := range uids {
		e1, err1 := s.readyDB.Exists(uid)
		if err1 == nil && !e1 {
			e2, err2 := s.failedDB.Exists(uid)
			if err2 == nil && !e2 {
				e3, err3 := s.permerrorsDB.Exists(uid)
				if err3 == nil && !e3 {
					orphaned_uids = append(orphaned_uids, uid)
				}
			}
		}
	}

	// if no match, delete the message
	for _, uid := range orphaned_uids {
		s.messagesDB.Delete(uid)
	}
}

func (s *MessageStore) resetStuckInSent() {
	// push back to "Ready" the messages that were sent out of the Store in the
	// last execution of skewer, but never were ACKed or NACKed
	uids := s.sentDB.ListKeys()
	s.logger.Debug("Pushing back stuck messages from Sent to Ready", "nb_messages", len(uids))
	s.sentDB.DeleteMany(uids)
	for _, uid := range uids {
		s.readyDB.Set(uid, []byte("true"))
	}

}

func (s *MessageStore) ReadAllBadgers() (map[string]string, map[string]string, map[string]string) {
	return nil, nil, nil // FIXME
}

func (s *MessageStore) resetFailures() {
	// push back messages from "failed" to "ready"
	s.failed_mu.Lock()
	for {
		now := time.Now()
		iter := s.failedDB.KeyValueIterator(1000)
		uids := []string{}
		invalidUids := []string{}
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
			errs, err := s.failedDB.DeleteMany(invalidUids)
			if s.metrics != nil {
				s.metrics.BadgerGauge.WithLabelValues("failed").Sub(float64(len(invalidUids) - len(errs)))
			}
			if err != nil {
				s.logger.Warn("Error deleting invalid entries", "error", err)
			}
		}

		if len(uids) == 0 {
			s.failed_mu.Unlock()
			return
		}

		s.ready_mu.Lock()
		readyBatch := map[string][]byte{}
		for _, uid := range uids {
			readyBatch[uid] = []byte("true")
		}
		errs, err := s.readyDB.AddMany(readyBatch)
		if err != nil {
			s.logger.Warn("Error pushing entries from failed queue to ready queue", "error", err)
		}
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("ready").Add(float64(len(readyBatch) - len(errs)))
		}

		if len(errs) < len(readyBatch) {
			for _, uid := range errs {
				delete(readyBatch, uid)
			}
			failedBatch := make([]string, 0, len(readyBatch))
			for uid := range readyBatch {
				failedBatch = append(failedBatch, uid)
			}
			errs, err = s.failedDB.DeleteMany(failedBatch)
			if err != nil {
				s.logger.Warn("Error deleting entries from failed queue", "error", err)
			}
			if s.metrics != nil {
				s.metrics.BadgerGauge.WithLabelValues("failed").Sub(float64(len(failedBatch) - len(errs)))
			}
			s.availMsgCond.Signal()
		}

		s.ready_mu.Unlock()
		s.failed_mu.Unlock()
	}
}

func (s *MessageStore) Stash(m model.TcpUdpParsedMessage) (fatal error, nonfatal error) {
	s.toStashQueue.Put(m)
	return nil, nil
}

func (s *MessageStore) ingest(queue []*model.TcpUdpParsedMessage) (int, error) {
	// we avoid "defer" as a performance optim

	if len(queue) == 0 {
		return 0, nil
	}

	marshalledQueue := map[string][]byte{}
	for _, m := range queue {
		b, err := m.MarshalMsg(nil)
		if err == nil {
			if len(b) == 0 {
				s.logger.Warn("Ingestion of empty message", "uid", m.Uid)
			} else {
				marshalledQueue[m.Uid] = b
			}
		} else {
			s.logger.Warn("The store discarded a message that could not be marshaled", "error", err)
		}
	}

	if len(marshalledQueue) == 0 {
		return 0, nil
	}

	s.ready_mu.Lock()
	s.messages_mu.Lock()

	errorMsgKeys, errMsg := s.messagesDB.AddMany(marshalledQueue)

	if len(errorMsgKeys) == len(marshalledQueue) {
		s.messages_mu.Unlock()
		s.ready_mu.Unlock()
		return 0, errMsg
	}

	s.metrics.BadgerGauge.WithLabelValues("messages").Add(float64(len(marshalledQueue) - len(errorMsgKeys)))

	for _, k := range errorMsgKeys {
		delete(marshalledQueue, k)
	}

	for k := range marshalledQueue {
		marshalledQueue[k] = []byte("true")
	}

	errReadyKeys, errReady := s.readyDB.AddMany(marshalledQueue)
	ingested := len(marshalledQueue) - len(errReadyKeys)
	if s.metrics != nil {
		s.metrics.BadgerGauge.WithLabelValues("ready").Add(float64(ingested))
	}
	if len(errReadyKeys) > 0 {
		s.messagesDB.DeleteMany(errReadyKeys)
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("messages").Sub(float64(len(errReadyKeys)))
		}
	}

	s.messages_mu.Unlock()
	if ingested > 0 {
		s.availMsgCond.Signal()
	}
	s.ready_mu.Unlock()

	if errMsg == nil {
		errMsg = errReady
	}

	return ingested, errMsg
}

func (s *MessageStore) retrieve(n int) (messages map[string]*model.TcpUdpParsedMessage) {
	s.messages_mu.Lock()

	messages = map[string]*model.TcpUdpParsedMessage{}

	iter := s.readyDB.KeyIterator(n)
	var fetched int = 0
	invalidEntries := []string{}
	var message *model.TcpUdpParsedMessage
	for iter.Rewind(); iter.Valid() && fetched < n; iter.Next() {
		uid := iter.Key()
		message_b, err := s.messagesDB.Get(uid)
		if err == nil {
			if len(message_b) > 0 {
				message = &model.TcpUdpParsedMessage{}
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
		}
	}
	iter.Close()

	if len(invalidEntries) > 0 {
		s.logger.Info("Found invalid entries", "number", len(invalidEntries))
		errs, err := s.readyDB.DeleteMany(invalidEntries)
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("ready").Sub(float64(len(invalidEntries) - len(errs)))
		}
		if err != nil {
			s.logger.Warn("Error deleting invalid entries from 'ready' queue", "error", err)
		}
		errs, err = s.messagesDB.DeleteMany(invalidEntries)
		if err != nil {
			s.logger.Warn("Error deleting invalid entries from 'messages' queue", "error", err)
		}
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("messages").Sub(float64(len(invalidEntries) - len(errs)))
		}
	}

	if len(messages) == 0 {
		s.messages_mu.Unlock()
		return messages
	}

	sentBatch := map[string][]byte{}
	for uid, _ := range messages {
		sentBatch[uid] = []byte("true")
	}
	errs, err := s.sentDB.AddMany(sentBatch)
	if s.metrics != nil {
		s.metrics.BadgerGauge.WithLabelValues("sent").Add(float64(len(sentBatch) - len(errs)))
	}
	if err != nil {
		s.logger.Warn("Error copying messages to the 'sent' queue", "error", err)
	}
	for _, errKey := range errs {
		delete(sentBatch, errKey)
	}
	readyBatch := make([]string, 0, len(sentBatch))
	for k := range sentBatch {
		readyBatch = append(readyBatch, k)
	}
	errs, err = s.readyDB.DeleteMany(readyBatch)
	if s.metrics != nil {
		s.metrics.BadgerGauge.WithLabelValues("ready").Sub(float64(len(readyBatch) - len(errs)))
	}
	if err != nil {
		s.logger.Warn("Error deleting messages from the 'ready' queue", "error", err)
	}

	for _, uid := range errs {
		delete(messages, uid)
	}
	s.messages_mu.Unlock()
	return messages
}

func (s *MessageStore) ACK(uid string) {
	s.ackQueue.Put(uid)
}

func (s *MessageStore) doACK(uids []string) {
	if len(uids) == 0 {
		return
	}
	s.messages_mu.Lock()
	errs, err := s.sentDB.DeleteMany(uids)
	if err != nil {
		s.logger.Warn("Error removing messages from the Sent DB", "error", err)
	}
	if len(errs) < len(uids) {
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("sent").Sub(float64(len(uids) - len(errs)))
		}
		uids_map := map[string]bool{}
		for _, uid := range uids {
			uids_map[uid] = true
		}
		for _, uid := range errs {
			delete(uids_map, uid)
		}
		uids = make([]string, 0, len(uids_map))
		for uid := range uids_map {
			uids = append(uids, uid)
		}
		errs, err := s.messagesDB.DeleteMany(uids)
		if err != nil {
			s.logger.Warn("Error removing message content from DB", "error", err)
		}
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("messages").Sub(float64(len(uids) - len(errs)))
		}
	}
	s.messages_mu.Unlock()
}

func (s *MessageStore) NACK(uid string) {
	s.nackQueue.Put(uid)
}

func (s *MessageStore) doNACK(uids []string) {
	if len(uids) == 0 {
		return
	}
	s.failed_mu.Lock()
	times := time.Now().Format(time.RFC3339)
	failedBatch := map[string][]byte{}
	for _, uid := range uids {
		failedBatch[uid] = []byte(times)
	}
	errs, err := s.failedDB.AddMany(failedBatch)
	if err != nil {
		s.logger.Warn("Error copying messages to the Failed DB", "error", err)
	}
	if len(errs) < len(failedBatch) {
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("failed").Add(float64(len(failedBatch) - len(errs)))
		}
		for _, uid := range errs {
			delete(failedBatch, uid)
		}
		uids = make([]string, 0, len(failedBatch))
		for uid := range failedBatch {
			uids = append(uids, uid)
		}
		errs, err := s.sentDB.DeleteMany(uids)
		if err != nil {
			s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		}
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("sent").Sub(float64(len(uids) - len(errs)))
		}
	}
	s.failed_mu.Unlock()
}

func (s *MessageStore) PermError(uid string) {
	s.permerrorsQueue.Put(uid)
}

func (s *MessageStore) doPermanentError(uids []string) {
	if len(uids) == 0 {
		return
	}
	times := time.Now().Format(time.RFC3339)
	permBatch := map[string][]byte{}
	for _, uid := range uids {
		permBatch[uid] = []byte(times)
	}
	errs, err := s.permerrorsDB.AddMany(permBatch)
	if err != nil {
		s.logger.Warn("Error copying messages to the PermErrors DB", "error", err)
	}
	if len(errs) < len(permBatch) {
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("permerrors").Add(float64(len(permBatch) - len(errs)))
		}
		for _, uid := range errs {
			delete(permBatch, uid)
		}
		uids = make([]string, 0, len(permBatch))
		for uid := range permBatch {
			uids = append(uids, uid)
		}
		errs, err := s.sentDB.DeleteMany(uids)
		if err != nil {
			s.logger.Warn("Error removing messages from the Sent DB", "error", err)
		}
		if s.metrics != nil {
			s.metrics.BadgerGauge.WithLabelValues("sent").Sub(float64(len(uids) - len(errs)))
		}
	}

}
