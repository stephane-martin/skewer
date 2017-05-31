package store

import (
	"encoding/json"
	"os"
	"path"
	"sync"
	"time"

	"github.com/dgraph-io/badger/badger"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/model"
)

type MessageStore struct {
	messages   *badger.KV
	ready      *badger.KV
	sent       *badger.KV
	failed     *badger.KV
	stopped_mu sync.Mutex
	ready_mu   sync.Mutex
	failed_mu  sync.Mutex
	stopped    bool
	Inputs     chan *model.TcpParsedMessage
	Outputs    chan *model.TcpParsedMessage
	wg         sync.WaitGroup
	ticker     *time.Ticker
	logger     log15.Logger
}

func NewStore(dirname string, l log15.Logger) (store *MessageStore, err error) {
	opts_messages := badger.DefaultOptions
	opts_ready := badger.DefaultOptions
	opts_sent := badger.DefaultOptions
	opts_failed := badger.DefaultOptions
	opts_messages.Dir = path.Join(dirname, "messages")
	opts_sent.Dir = path.Join(dirname, "sent")
	opts_ready.Dir = path.Join(dirname, "ready")
	opts_failed.Dir = path.Join(dirname, "failed")
	err = os.MkdirAll(opts_messages.Dir, 0700)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(opts_sent.Dir, 0700)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(opts_ready.Dir, 0700)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(opts_failed.Dir, 0700)
	if err != nil {
		return nil, err
	}

	s := MessageStore{}
	store = &s

	store.messages, err = badger.NewKV(&opts_messages)
	if err != nil {
		return nil, err
	}
	store.ready, err = badger.NewKV(&opts_ready)
	if err != nil {
		return nil, err
	}
	store.sent, err = badger.NewKV(&opts_sent)
	if err != nil {
		return nil, err
	}
	store.failed, err = badger.NewKV(&opts_failed)
	if err != nil {
		return nil, err
	}
	store.logger = l.New("class", "MessageStore")

	// only once, push back messages from previous run that may have been stuck in the sent queue
	s.resetStuckInSent()

	store.ticker = time.NewTicker(time.Minute)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.logger.Debug("End of periodic Store.resetFailures")
		for {
			select {
			case <-store.ticker.C:
				if store.Stopped() {
					return
				}
				store.resetFailures()
			case <-time.After(time.Second):
				if store.Stopped() {
					return
				}
			}
		}
	}()
	s.startIngest()

	return store, nil
}

func (s *MessageStore) StopSend() {
	s.stopped_mu.Lock()
	if s.stopped {
		s.stopped_mu.Unlock()
		return
	}
	close(s.Inputs)
	s.stopped = true // indicates that s.Inputs has been closed: no more incoming messages
	s.ticker.Stop()
	s.stopped_mu.Unlock()

	s.wg.Wait() // wait that ingest and StartSend have finished
}

func (s *MessageStore) Close() {
	s.StopSend()
	s.messages.Close()
	s.ready.Close()
	s.sent.Close()
	s.failed.Close()
}

func (s *MessageStore) Stopped() bool {
	s.stopped_mu.Lock()
	defer s.stopped_mu.Unlock()
	return s.stopped
}

func (s *MessageStore) resetStuckInSent() {
	iter_opts := badger.IteratorOptions{
		PrefetchSize: 1000,
		FetchValues:  false,
		Reverse:      false,
	}

	uids := []string{}
	iter := s.sent.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		uid := string(item.Key())
		uids = append(uids, uid)
	}
	for _, uid := range uids {
		s.sent.Delete([]byte(uid))
		s.ready.Set([]byte(uid), []byte("true"))
	}
}

func (s *MessageStore) resetFailures() {
	s.logger.Debug("resetFailures")
	// push back messages from "failed" to "ready"
	iter_opts := badger.IteratorOptions{
		PrefetchSize: 1000,
		FetchValues:  true,
		Reverse:      false,
	}
	for {
		s.failed_mu.Lock()
		now := time.Now()
		iter := s.failed.NewIterator(iter_opts)
		fetched := 0
		uids := []string{}
		for iter.Rewind(); iter.Valid() && fetched < 1000; iter.Next() {
			item := iter.Item()
			uid := string(item.Key())
			time_s := string(item.Value())
			t, err := time.Parse(time.RFC3339, time_s)
			if err == nil {
				if now.Sub(t) >= time.Minute {
					// messages that failed to be delivered to Kafka should be tried again after 1 minute
					uids = append(uids, uid)
				}
			}
		}
		if len(uids) == 0 {
			s.failed_mu.Unlock()
			break
		}
		s.ready_mu.Lock()
		// todo: batch
		for _, uid := range uids {
			s.failed.Delete([]byte(uid))
			s.ready.Set([]byte(uid), []byte("true"))
		}
		s.ready_mu.Unlock()
		s.failed_mu.Unlock()
		s.logger.Debug("Failed messages pushed back to ready state", "nb_messages", len(uids))
	}
}

func (s *MessageStore) startIngest() {
	s.logger.Debug("startIngest")
	s.Inputs = make(chan *model.TcpParsedMessage, 10000)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for m := range s.Inputs {
			s.stash(m)
		}
		s.logger.Debug("ingestion goroutine has finished")
	}()
}

func (s *MessageStore) StartSend() {
	s.logger.Debug("StartSend")
	s.Outputs = make(chan *model.TcpParsedMessage)
	// todo: ticker to periodically reset failures
	s.wg.Add(1)
	go func() {
		defer func() {
			close(s.Outputs)
			s.wg.Done()
		}()
		for {
			messages := s.retrieve(1000)
			if len(messages) == 0 {
				if s.Stopped() {
					s.logger.Debug("sending goroutine has ended")
					return
				}
				time.Sleep(time.Duration(200) * time.Millisecond)
			} else {
				for _, m := range messages {
					s.Outputs <- m
				}
			}
		}
	}()
}

func (s *MessageStore) stash(m *model.TcpParsedMessage) {

	b, err := json.Marshal(m)
	if err == nil {
		s.ready_mu.Lock()
		defer s.ready_mu.Unlock()
		s.messages.Set([]byte(m.Uid), b)
		s.ready.Set([]byte(m.Uid), []byte("true"))
	}
}

func (s *MessageStore) retrieve(n int) (messages map[string]*model.TcpParsedMessage) {
	s.ready_mu.Lock()
	defer s.ready_mu.Unlock()
	messages = map[string]*model.TcpParsedMessage{}
	iter_opts := badger.IteratorOptions{
		PrefetchSize: n,
		FetchValues:  false,
		Reverse:      false,
	}
	iter := s.ready.NewIterator(iter_opts)
	fetched := 0
	for iter.Rewind(); iter.Valid() && fetched < n; iter.Next() {
		uid := iter.Item().Key()
		item := badger.KVItem{}
		err := s.messages.Get(uid, &item)
		if err == nil {
			message_b := item.Value()
			if message_b != nil {
				message := model.TcpParsedMessage{}
				err := json.Unmarshal(message_b, &message)
				if err == nil {
					messages[string(uid)] = &message
					fetched++
				}
			}
		} else {
			s.logger.Warn("Error getting message from internal DB", "uid", string(uid))
		}
	}
	// todo: use batch
	for uid, _ := range messages {
		s.sent.Set([]byte(uid), []byte("true"))
		s.ready.Delete([]byte(uid))
	}
	return messages
}

func (s *MessageStore) Ack(id string) {
	s.sent.Delete([]byte(id))
	s.messages.Delete([]byte(id))
}

func (s *MessageStore) Nack(id string) {
	s.failed_mu.Lock()
	defer s.failed_mu.Unlock()
	s.sent.Delete([]byte(id))
	s.failed.Set([]byte(id), []byte(time.Now().Format(time.RFC3339)))
}
