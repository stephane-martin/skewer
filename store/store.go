package store

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

type MessageStore struct {
	badger          *badger.KV
	messagesDB      utils.Partition
	readyDB         utils.Partition
	sentDB          utils.Partition
	failedDB        utils.Partition
	permerrorsDB    utils.Partition
	syslogConfigsDB utils.Partition

	metrics *metrics.Metrics

	ready_mu     *sync.Mutex
	failed_mu    *sync.Mutex
	messages_mu  *sync.Mutex
	availMsgCond *sync.Cond
	wg           *sync.WaitGroup

	ticker *time.Ticker
	logger log15.Logger

	closedChan     chan struct{}
	FatalErrorChan chan struct{}

	InputsChan  chan *model.TcpUdpParsedMessage
	OutputsChan chan *model.TcpUdpParsedMessage
	AckChan     chan string
	NackChan    chan string
	ErrorsChan  chan string
}

func (s *MessageStore) Inputs() chan *model.TcpUdpParsedMessage {
	return s.InputsChan
}

func (s *MessageStore) Outputs() chan *model.TcpUdpParsedMessage {
	return s.OutputsChan
}

func (s *MessageStore) Ack() chan string {
	return s.AckChan
}

func (s *MessageStore) Nack() chan string {
	return s.NackChan
}

func (s *MessageStore) ProcessingErrors() chan string {
	return s.ErrorsChan
}

func (s *MessageStore) Errors() chan struct{} {
	return s.FatalErrorChan
}

func NewStore(ctx context.Context, cfg conf.StoreConfig, m *metrics.Metrics, l log15.Logger) (Store, error) {

	badgerOpts := badger.DefaultOptions
	badgerOpts.Dir = cfg.Dirname
	badgerOpts.ValueDir = cfg.Dirname
	badgerOpts.MaxTableSize = cfg.Maxsize
	badgerOpts.SyncWrites = cfg.FSync

	err := os.MkdirAll(cfg.Dirname, 0700)
	if err != nil {
		return nil, err
	}

	store := &MessageStore{metrics: m}
	store.logger = l.New("class", "MessageStore")

	store.ready_mu = &sync.Mutex{}
	store.failed_mu = &sync.Mutex{}
	store.messages_mu = &sync.Mutex{}
	store.wg = &sync.WaitGroup{}
	store.availMsgCond = sync.NewCond(store.ready_mu)

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

	store.InputsChan = make(chan *model.TcpUdpParsedMessage, 10000)
	store.FatalErrorChan = make(chan struct{})
	store.AckChan = make(chan string)
	store.NackChan = make(chan string)
	store.ErrorsChan = make(chan string)
	store.ticker = time.NewTicker(time.Minute)

	store.wg.Add(1)
	go func() {
		defer store.wg.Done()
		for {
			if store.AckChan == nil && store.NackChan == nil && store.ErrorsChan == nil {
				return
			}
			select {
			case uid, more := <-store.AckChan:
				if more {
					// message has been ACKed by Kafka, we can delete it from the Store
					store.doACK(uid)
				} else {
					store.AckChan = nil
				}
			case uid, more := <-store.NackChan:
				if more {
					// message has been NACked by Kafka
					// it will be retried later
					store.doNACK(uid)
				} else {
					store.NackChan = nil
				}
			case uid, more := <-store.ErrorsChan:
				if more {
					// the message was not transmitted to Kafka, because the preprocessing failed
					// it is a permanent error: if we retried, the same error would happen again
					// so we move the message to a "permanent errors" DB
					// it is the operator job to decide what to do with them (manually)
					store.doPermanentError(uid)
				} else {
					store.ErrorsChan = nil
				}
			}

		}
	}()

	store.wg.Add(1)
	go func() {
		defer func() {
			store.ticker.Stop()
			store.wg.Done()
		}()
		done := ctx.Done()
		for {
			select {
			case msg, more := <-store.InputsChan:
				if more {
					err = store.stash(msg)
					if err != nil {
						if err == badger.ErrNoRoom {
							store.logger.Crit("The store is full!")
							close(store.FatalErrorChan) // signal the caller service than we should stop everything
						} else {
							store.logger.Warn("Store unexpected error", "error", err)
						}
					}
				} else {
					return
				}

			case <-store.ticker.C:
				store.resetFailures()
			case <-done:
				close(store.InputsChan)
				done = nil
			}

		}
	}()

	store.OutputsChan = make(chan *model.TcpUdpParsedMessage)

	store.wg.Add(1)
	go func() {
		<-ctx.Done()
		// if the next goroutine is blocked in availMsgCond.Wait(), unblock it
		store.availMsgCond.Signal()
		store.wg.Done()
	}()

	store.wg.Add(1)
	go func() {
		doneChan := ctx.Done()
		store.ready_mu.Lock()
		defer func() {
			store.ready_mu.Unlock()
			close(store.OutputsChan)
			store.wg.Done()
		}()
		for {
			// return if the context was canceled
			select {
			case <-doneChan:
				return
			default:
			}

			messages := store.retrieve(1000)
			// loop on the available messages, but immediately stop if the context is canceled
			for _, msg := range messages {
				select {
				case store.OutputsChan <- msg:
				case <-doneChan:
					return
				}
			}
			// if there is no available message, wait for more
			if len(messages) == 0 {
				store.availMsgCond.Wait() // during Wait(), ready_mu is unlocked. After Wait() returns, ready_mu is locked again.
			}
		}
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

func (s *MessageStore) StoreSyslogConfig(config *conf.SyslogConfig) (configID string, err error) {
	data := config.Export()
	h := sha512.Sum512(data)
	confID := base64.StdEncoding.EncodeToString(h[:])
	exists, err := s.syslogConfigsDB.Exists(confID)
	if err != nil {
		return "", err
	}
	if !exists {
		err = s.syslogConfigsDB.Set(confID, data)
		if err != nil {
			return "", err
		}
		s.metrics.BadgerGauge.WithLabelValues("syslogconf").Inc()
	}
	return confID, nil
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
	s.metrics.BadgerGauge.WithLabelValues("messages").Set(float64(s.messagesDB.Count()))
	s.metrics.BadgerGauge.WithLabelValues("ready").Set(float64(s.readyDB.Count()))
	s.metrics.BadgerGauge.WithLabelValues("sent").Set(float64(s.sentDB.Count()))
	s.metrics.BadgerGauge.WithLabelValues("failed").Set(float64(s.failedDB.Count()))
	s.metrics.BadgerGauge.WithLabelValues("permerrors").Set(float64(s.permerrorsDB.Count()))
	s.metrics.BadgerGauge.WithLabelValues("syslogconf").Set(float64(s.syslogConfigsDB.Count()))
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
	// last execution of skewer, but never ACKed or NACKed
	uids := s.sentDB.ListKeys()
	s.logger.Debug("Pushing back stuck messages from Sent to Ready", "nb_messages", len(uids))
	s.sentDB.DeleteKeys(uids)
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
				// todo
			}
		}
		iter.Close()

		if len(uids) == 0 {
			s.failed_mu.Unlock()
			return
		}

		s.ready_mu.Lock()
		for _, uid := range uids {
			err := s.readyDB.Set(uid, []byte("true"))
			if err != nil {
				s.logger.Warn("Error pushing entry from failed queue to ready queue", "uid", uid, "error", err)
			} else {
				s.metrics.BadgerGauge.WithLabelValues("ready").Inc()
				err := s.failedDB.Delete(uid)
				if err != nil {
					s.logger.Warn("Error deleting entry from failed queue", "uid", uid, "error", err)
				} else {
					s.metrics.BadgerGauge.WithLabelValues("failed").Dec()
				}
			}
		}
		s.availMsgCond.Signal()
		s.ready_mu.Unlock()
		s.failed_mu.Unlock()
	}
}

func (s *MessageStore) stash(m *model.TcpUdpParsedMessage) error {
	// we avoid "defer" as a performance optim
	b, err := json.Marshal(m)
	if err != nil {
		s.logger.Warn("The store discarded a message that could not be JSON-marshalled", "error", err)
		return nil
	}
	s.ready_mu.Lock()
	s.messages_mu.Lock()
	err = s.messagesDB.Set(m.Uid, b)
	if err != nil {
		s.messages_mu.Unlock()
		s.ready_mu.Unlock()
		return errwrap.Wrapf("Error writing message content: {{err}}", err)
	}
	s.metrics.BadgerGauge.WithLabelValues("messages").Inc()
	err = s.readyDB.Set(m.Uid, []byte("true"))
	if err != nil {
		s.messagesDB.Delete(m.Uid)
		s.messages_mu.Unlock()
		s.ready_mu.Unlock()
		return errwrap.Wrapf("Error writing message to Ready: {{err}}", err)
	}
	s.metrics.BadgerGauge.WithLabelValues("ready").Inc()
	s.messages_mu.Unlock()
	s.availMsgCond.Signal()
	s.ready_mu.Unlock()
	return nil
}

func (s *MessageStore) retrieve(n int) (messages map[string]*model.TcpUdpParsedMessage) {
	s.messages_mu.Lock()

	messages = map[string]*model.TcpUdpParsedMessage{}

	iter := s.readyDB.KeyIterator(n)
	var fetched int = 0
	invalidEntries := []string{}
	for iter.Rewind(); iter.Valid() && fetched < n; iter.Next() {
		uid := iter.Key()
		message_b, err := s.messagesDB.Get(uid)
		if err == nil {
			if message_b != nil {
				message := model.TcpUdpParsedMessage{}
				err := json.Unmarshal(message_b, &message)
				if err == nil {
					messages[uid] = &message
					fetched++
				} else {
					invalidEntries = append(invalidEntries, uid)
				}
			} else {
				invalidEntries = append(invalidEntries, uid)
			}
		} else {
			s.logger.Warn("Error getting message content from message queue", "uid", uid)
		}
	}
	iter.Close()

	for _, uid := range invalidEntries {
		s.logger.Debug("Deleting invalid entry", "uid", string(uid))
		err := s.readyDB.Delete(uid)
		if err == nil {
			s.metrics.BadgerGauge.WithLabelValues("ready").Dec()
			err = s.messagesDB.Delete(uid)
			if err == nil {
				s.metrics.BadgerGauge.WithLabelValues("messages").Dec()
			}
		}
	}

	if len(messages) == 0 {
		s.messages_mu.Unlock()
		return messages
	}

	var err error
	uidsKO := []string{}
	for uid, _ := range messages {
		// todo: batch
		err = s.sentDB.Set(uid, []byte("true"))
		if err != nil {
			s.logger.Warn("Error copying messages from Ready to Sent", "uid", uid, "error", err)
			uidsKO = append(uidsKO, uid)
		} else {
			s.metrics.BadgerGauge.WithLabelValues("sent").Inc()
			err = s.readyDB.Delete(uid)
			if err != nil {
				s.logger.Warn("Error deleting messages from Ready", "uid", uid, "error", err)
			} else {
				s.metrics.BadgerGauge.WithLabelValues("ready").Dec()
			}
		}
	}
	for _, uid := range uidsKO {
		delete(messages, uid)
	}
	s.messages_mu.Unlock()
	return messages
}

func (s *MessageStore) doACK(uid string) {
	s.messages_mu.Lock()
	err := s.sentDB.Delete(uid)
	if err != nil {
		s.logger.Warn("Error removing message from the Sent DB", "uid", uid, "error", err)
	} else {
		s.metrics.BadgerGauge.WithLabelValues("sent").Dec()
		err := s.messagesDB.Delete(uid)
		if err != nil {
			s.logger.Warn("Error removing message content from DB", "uid", uid, "error", err)
		} else {
			s.metrics.BadgerGauge.WithLabelValues("messages").Dec()
		}
	}
	s.messages_mu.Unlock()
}

func (s *MessageStore) doNACK(uid string) {
	s.failed_mu.Lock()
	err := s.failedDB.Set(uid, []byte(time.Now().Format(time.RFC3339)))
	if err != nil {
		s.logger.Warn("Error moving the message to the Failed DB", "uid", uid, "error", err)
	} else {
		s.metrics.BadgerGauge.WithLabelValues("failed").Inc()
		err := s.sentDB.Delete(uid)
		if err != nil {
			s.logger.Warn("Error removing message from the Sent DB", "uid", uid, "error", err)
		} else {
			s.metrics.BadgerGauge.WithLabelValues("sent").Dec()
		}
	}
	s.failed_mu.Unlock()
}

func (s *MessageStore) doPermanentError(uid string) {
	err := s.permerrorsDB.Set(uid, []byte(time.Now().Format(time.RFC3339)))
	if err != nil {
		s.logger.Warn("Error moving the message to the Permanent Errors DB", "uid", uid, "error", err)
	} else {
		s.metrics.BadgerGauge.WithLabelValues("permerrors").Inc()
		err := s.sentDB.Delete(uid)
		if err != nil {
			s.logger.Warn("Error removing message from the Sent DB", "uid", uid, "error", err)
		} else {
			s.metrics.BadgerGauge.WithLabelValues("sent").Dec()
		}
	}
}
