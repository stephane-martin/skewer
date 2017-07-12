package store

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/dgraph-io/badger"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

type MessageStore struct {
	messagesDB      utils.DB
	readyDB         *badger.KV
	sentDB          *badger.KV
	failedDB        *badger.KV
	syslogConfigsDB *badger.KV
	ready_mu        *sync.Mutex
	failed_mu       *sync.Mutex
	messages_mu     *sync.Mutex
	wg              *sync.WaitGroup
	ticker          *time.Ticker
	logger          log15.Logger
	closedChan      chan struct{}
	FatalErrorChan  chan struct{}
	InputsChan      chan *model.TcpUdpParsedMessage
	OutputsChan     chan *model.TcpUdpParsedMessage
	AckChan         chan string
	NackChan        chan string
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

func (s *MessageStore) Errors() chan struct{} {
	return s.FatalErrorChan
}

type Store interface {
	Inputs() chan *model.TcpUdpParsedMessage
	Outputs() chan *model.TcpUdpParsedMessage
	Ack() chan string
	Nack() chan string
	Errors() chan struct{}
	WaitFinished()
	GetSyslogConfig(configID string) (*conf.SyslogConfig, error)
	StoreSyslogConfig(config *conf.SyslogConfig) (string, error)
	ReadAllBadgers() (map[string]string, map[string]string, map[string]string)
}

type Forwarder interface {
	Forward(ctx context.Context, from Store, to conf.KafkaConfig) bool
	ErrorChan() chan struct{}
	WaitFinished()
}

func NewForwarder(test bool, m *metrics.Metrics, logger log15.Logger) (fwder Forwarder) {
	if test {
		dummy := dummyKafkaForwarder{logger: logger.New("class", "dummyKafkaForwarder"), metrics: m}
		dummy.errorChan = make(chan struct{})
		dummy.wg = &sync.WaitGroup{}
		fwder = &dummy
	} else {
		notdummy := kafkaForwarder{logger: logger.New("class", "kafkaForwarder"), metrics: m}
		notdummy.errorChan = make(chan struct{})
		notdummy.wg = &sync.WaitGroup{}
		fwder = &notdummy
	}
	return fwder
}

type kafkaForwarder struct {
	logger     log15.Logger
	errorChan  chan struct{}
	wg         *sync.WaitGroup
	forwarding int32
	metrics    *metrics.Metrics
}

func (fwder *kafkaForwarder) ErrorChan() chan struct{} {
	return fwder.errorChan
}

func (fwder *kafkaForwarder) WaitFinished() {
	fwder.wg.Wait()
}

type dummyKafkaForwarder struct {
	logger     log15.Logger
	errorChan  chan struct{}
	wg         *sync.WaitGroup
	forwarding int32
	metrics    *metrics.Metrics
}

func (fwder *dummyKafkaForwarder) ErrorChan() chan struct{} {
	return fwder.errorChan
}

func (fwder *dummyKafkaForwarder) WaitFinished() {
	fwder.wg.Wait()
}

func (fwder *kafkaForwarder) Forward(ctx context.Context, from Store, to conf.KafkaConfig) bool {
	// ensure Forward is only executing once
	if !atomic.CompareAndSwapInt32(&fwder.forwarding, 0, 1) {
		return false
	}
	go fwder.doForward(ctx, from, to)
	return true
}

func (fwder *dummyKafkaForwarder) Forward(ctx context.Context, from Store, to conf.KafkaConfig) bool {
	// ensure Forward is only executing once
	if !atomic.CompareAndSwapInt32(&fwder.forwarding, 0, 1) {
		return false
	}
	go fwder.doForward(ctx, from, to)
	return true
}

func (fwder *kafkaForwarder) doForward(ctx context.Context, from Store, to conf.KafkaConfig) {

	fwder.errorChan = make(chan struct{})
	ackChan := from.Ack()
	nackChan := from.Nack()
	outputsChan := from.Outputs()

	jsenvs := map[string]javascript.FilterEnvironment{}

	var producer sarama.AsyncProducer
	var err error
	for {
		producer, err = to.GetAsyncProducer()
		if err == nil {
			fwder.logger.Debug("Got a Kafka producer")
			break
		} else {
			fwder.metrics.KafkaConnectionErrorCounter.Inc()
			fwder.logger.Warn("Error getting a Kafka client", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
	wg := &sync.WaitGroup{}
	defer producer.AsyncClose()
	succChan := producer.Successes()
	failChan := producer.Errors()
	once := &sync.Once{}

	// listen for kafka responses
	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			atomic.StoreInt32(&fwder.forwarding, 0)
		}()

		for {
			if succChan == nil && failChan == nil {
				return
			}
			select {
			case succ, more := <-succChan:
				if more {
					ackChan <- succ.Metadata.(string)
					fwder.metrics.KafkaAckNackCounter.WithLabelValues("ack", succ.Topic).Inc()
				} else {
					succChan = nil
				}

			case fail, more := <-failChan:
				if more {
					nackChan <- fail.Msg.Metadata.(string)
					fwder.logger.Info("Kafka producer error", "error", fail.Error())
					if model.IsFatalKafkaError(fail.Err) {
						once.Do(func() { close(fwder.errorChan) })
					}
					fwder.metrics.KafkaAckNackCounter.WithLabelValues("nack", fail.Msg.Topic).Inc()
				} else {
					failChan = nil
				}
			}
		}
	}()

ForOutputs:
	for {
		select {
		case <-ctx.Done():
			return
		case message, more := <-outputsChan:
			if !more {
				return
			}
			env, ok := jsenvs[message.ConfId]
			if !ok {
				config, err := from.GetSyslogConfig(message.ConfId)
				if err != nil {
					// todo: log
					continue ForOutputs
				}
				jsenvs[message.ConfId] = javascript.NewFilterEnvironment(
					config.FilterFunc,
					config.TopicFunc,
					config.TopicTmpl,
					config.PartitionFunc,
					config.PartitionTmpl,
					fwder.logger,
				)
				env = jsenvs[message.ConfId]
			}

			topic, errs := env.Topic(message.Parsed.Fields)
			for _, err := range errs {
				fwder.logger.Info("Error calculating topic", "error", err, "uid", message.Uid)
			}
			partitionKey, errs := env.PartitionKey(message.Parsed.Fields)
			for _, err := range errs {
				fwder.logger.Info("Error calculating the partition key", "error", err, "uid", message.Uid)
			}

			if len(topic) == 0 || len(partitionKey) == 0 {
				fwder.logger.Warn("Topic or PartitionKey could not be calculated", "uid", message.Uid)
				nackChan <- message.Uid
				continue ForOutputs
			}

			tmsg, filterResult, err := env.FilterMessage(message.Parsed.Fields)

			switch filterResult {
			case javascript.DROPPED:
				ackChan <- message.Uid
				fwder.metrics.MessageFilteringCounter.WithLabelValues("dropped", message.Parsed.Client).Inc()
				continue ForOutputs
			case javascript.REJECTED:
				fwder.metrics.MessageFilteringCounter.WithLabelValues("rejected", message.Parsed.Client).Inc()
				nackChan <- message.Uid
				continue ForOutputs
			case javascript.PASS:
				fwder.metrics.MessageFilteringCounter.WithLabelValues("passing", message.Parsed.Client).Inc()
				if tmsg == nil {
					ackChan <- message.Uid
					continue ForOutputs
				}
			default:
				nackChan <- message.Uid
				fwder.logger.Warn("Error happened processing message", "uid", message.Uid, "error", err)
				fwder.metrics.MessageFilteringCounter.WithLabelValues("unknown", message.Parsed.Client).Inc()
				// todo: log the faulty message to a specific log
				continue ForOutputs
			}

			nmsg := model.ParsedMessage{
				Fields:         tmsg,
				Client:         message.Parsed.Client,
				LocalPort:      message.Parsed.LocalPort,
				UnixSocketPath: message.Parsed.UnixSocketPath,
			}

			kafkaMsg, err := nmsg.ToKafkaMessage(partitionKey, topic)
			if err != nil {
				fwder.logger.Warn("Error generating Kafka message", "error", err, "uid", message.Uid)
				nackChan <- message.Uid
				continue ForOutputs
			}

			kafkaMsg.Metadata = message.Uid
			producer.Input() <- kafkaMsg
		}
	}
}

func (fwder *dummyKafkaForwarder) doForward(ctx context.Context, from Store, to conf.KafkaConfig) {
	fwder.errorChan = make(chan struct{})
	jsenvs := map[string]javascript.FilterEnvironment{}
	ackChan := from.Ack()
	nackChan := from.Nack()
	outputsChan := from.Outputs()

	defer atomic.StoreInt32(&fwder.forwarding, 0)

ForOutputs:
	for {
		select {
		case <-ctx.Done():
			return
		case message, more := <-outputsChan:
			if !more {
				return
			}
			env, ok := jsenvs[message.ConfId]
			if !ok {
				config, err := from.GetSyslogConfig(message.ConfId)
				if err != nil {
					// todo: log
					continue ForOutputs
				}
				jsenvs[message.ConfId] = javascript.NewFilterEnvironment(
					config.FilterFunc,
					config.TopicFunc,
					config.TopicTmpl,
					config.PartitionFunc,
					config.PartitionTmpl,
					fwder.logger,
				)
				env = jsenvs[message.ConfId]
			}

			if message != nil {
				topic, errs := env.Topic(message.Parsed.Fields)
				for _, err := range errs {
					fwder.logger.Info("Error calculating topic", "error", err, "uid", message.Uid)
				}
				partitionKey, errs := env.PartitionKey(message.Parsed.Fields)
				for _, err := range errs {
					fwder.logger.Info("Error calculating the partition key", "error", err, "uid", message.Uid)
				}

				if len(topic) == 0 || len(partitionKey) == 0 {
					fwder.logger.Warn("Topic or PartitionKey could not be calculated", "uid", message.Uid)
					nackChan <- message.Uid
					continue ForOutputs
				}

				tmsg, filterResult, err := env.FilterMessage(message.Parsed.Fields)

				switch filterResult {
				case javascript.DROPPED:
					ackChan <- message.Uid
					fwder.metrics.MessageFilteringCounter.WithLabelValues("dropped", message.Parsed.Client).Inc()
					continue ForOutputs
				case javascript.REJECTED:
					nackChan <- message.Uid
					fwder.metrics.MessageFilteringCounter.WithLabelValues("rejected", message.Parsed.Client).Inc()
					continue ForOutputs
				case javascript.PASS:
					fwder.metrics.MessageFilteringCounter.WithLabelValues("passing", message.Parsed.Client).Inc()
					if tmsg == nil {
						ackChan <- message.Uid
						continue ForOutputs
					}
				default:
					ackChan <- message.Uid
					fwder.logger.Warn("Error happened when processing message", "uid", message.Uid, "error", err)
					fwder.metrics.MessageFilteringCounter.WithLabelValues("unknown", message.Parsed.Client).Inc()
					// todo: log the faulty message to a specific log
					continue ForOutputs
				}

				nmsg := model.ParsedMessage{
					Fields:         tmsg,
					Client:         message.Parsed.Client,
					LocalPort:      message.Parsed.LocalPort,
					UnixSocketPath: message.Parsed.UnixSocketPath,
				}
				kafkaMsg, err := nmsg.ToKafkaMessage(partitionKey, topic)
				if err != nil {
					fwder.logger.Warn("Error generating Kafka message", "error", err, "uid", message.Uid)
					nackChan <- message.Uid
					continue ForOutputs
				}

				v, _ := kafkaMsg.Value.Encode()
				pkey, _ := kafkaMsg.Key.Encode()
				fmt.Printf("pkey: '%s' topic:'%s' uid:'%s'\n", pkey, kafkaMsg.Topic, message.Uid)
				fmt.Println(string(v))
				fmt.Println()

				ackChan <- message.Uid
			}
		}
	}
}

func NewStore(ctx context.Context, cfg conf.StoreConfig, l log15.Logger) (Store, error) {

	opts_messages := badger.DefaultOptions
	opts_ready := badger.DefaultOptions
	opts_sent := badger.DefaultOptions
	opts_failed := badger.DefaultOptions
	opts_configs := badger.DefaultOptions
	opts_messages.Dir = path.Join(cfg.Dirname, "messages")
	opts_messages.ValueDir = path.Join(cfg.Dirname, "messages")
	opts_sent.Dir = path.Join(cfg.Dirname, "sent")
	opts_sent.ValueDir = path.Join(cfg.Dirname, "sent")
	opts_ready.Dir = path.Join(cfg.Dirname, "ready")
	opts_ready.ValueDir = path.Join(cfg.Dirname, "ready")
	opts_failed.Dir = path.Join(cfg.Dirname, "failed")
	opts_failed.ValueDir = path.Join(cfg.Dirname, "failed")
	opts_configs.Dir = path.Join(cfg.Dirname, "configs")
	opts_configs.ValueDir = path.Join(cfg.Dirname, "configs")

	opts_messages.MaxTableSize = cfg.Maxsize
	opts_messages.SyncWrites = cfg.FSync
	opts_ready.SyncWrites = cfg.FSync
	opts_sent.SyncWrites = cfg.FSync
	opts_failed.SyncWrites = cfg.FSync
	opts_configs.SyncWrites = true

	err := os.MkdirAll(opts_messages.Dir, 0700)
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
	err = os.MkdirAll(opts_configs.Dir, 0700)
	if err != nil {
		return nil, err
	}

	store := &MessageStore{}
	store.logger = l.New("class", "MessageStore")
	store.ready_mu = &sync.Mutex{}
	store.failed_mu = &sync.Mutex{}
	store.messages_mu = &sync.Mutex{}
	store.wg = &sync.WaitGroup{}
	store.closedChan = make(chan struct{})

	if len(cfg.Secret) > 0 {
		store.logger.Info("The Store is encrypted")
		store.messagesDB, err = utils.NewEncryptedDB(&opts_messages, cfg.SecretB)
	} else {
		store.messagesDB, err = utils.NewNonEncryptedDB(&opts_messages)
	}
	if err != nil {
		return nil, err
	}

	store.readyDB, err = badger.NewKV(&opts_ready)
	if err != nil {
		return nil, err
	}
	store.sentDB, err = badger.NewKV(&opts_sent)
	if err != nil {
		return nil, err
	}
	store.failedDB, err = badger.NewKV(&opts_failed)
	if err != nil {
		return nil, err
	}
	store.syslogConfigsDB, err = badger.NewKV(&opts_configs)
	if err != nil {
		return nil, err
	}

	// only once, push back messages from previous run that may have been stuck in the sent queue
	store.resetStuckInSent()

	// prune orphaned messages
	store.pruneOrphaned()

	store.InputsChan = make(chan *model.TcpUdpParsedMessage, 10000)
	store.FatalErrorChan = make(chan struct{})
	store.AckChan = make(chan string)
	store.NackChan = make(chan string)
	store.ticker = time.NewTicker(time.Minute)

	store.wg.Add(1)
	go func() {
		defer store.wg.Done()
		for {
			if store.AckChan == nil && store.NackChan == nil {
				return
			}
			select {
			case uid, more := <-store.AckChan:
				if more {
					store.doACK(uid)
				} else {
					store.AckChan = nil
				}
			case uid, more := <-store.NackChan:
				if more {
					store.doNACK(uid)
				} else {
					store.NackChan = nil
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
		defer func() {
			close(store.OutputsChan)
			store.wg.Done()
		}()
		for {
			messages := store.retrieve(1000)
			if len(messages) == 0 {
				select {
				case <-time.After(1000 * time.Millisecond):
				case <-ctx.Done():
					return
				}
			} else {
				for _, msg := range messages {
					select {
					case store.OutputsChan <- msg:
					case <-ctx.Done():
						return
					}
				}
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
	hs := h[:]
	exists, err := s.syslogConfigsDB.Exists(hs)
	if err != nil {
		return "", err
	}
	if !exists {
		err := s.syslogConfigsDB.Set(hs, data)
		if err != nil {
			return "", err
		}
	}
	return base64.StdEncoding.EncodeToString(hs), nil
}

func (s *MessageStore) GetSyslogConfig(configID string) (*conf.SyslogConfig, error) {
	ident, err := base64.StdEncoding.DecodeString(configID)
	if err != nil {
		return nil, fmt.Errorf("The id can't be decoded")
	}
	kv := badger.KVItem{}
	s.syslogConfigsDB.Get(ident, &kv)
	data := kv.Value()
	if data == nil {
		return nil, fmt.Errorf("Unknown syslog configuration id")
	}
	c, err := conf.ImportSyslogConfig(data)
	if err != nil {
		return nil, fmt.Errorf("Can't unmarshal the syslog config: %s", err.Error())
	}
	return c, nil
}

func (s *MessageStore) closeBadgers() {
	err := s.messagesDB.Close()
	if err != nil {
		s.logger.Warn("Error closing 'Messages' store", "error", err)
	}
	err = s.readyDB.Close()
	if err != nil {
		s.logger.Warn("Error closing 'Ready' store", "error", err)
	}
	err = s.sentDB.Close()
	if err != nil {
		s.logger.Warn("Error closing 'Sent' store", "error", err)
	}
	err = s.failedDB.Close()
	if err != nil {
		s.logger.Warn("Error closing 'Failed' store", "error", err)
	}
	s.logger.Info("Badger databases are closed")
}

func (s *MessageStore) pruneOrphaned() {
	// find if we have some old full messages

	uids := s.messagesDB.ListKeys()

	// check if the corresponding uid exists in "ready" or "failed"
	orphaned_uids := []string{}
	for _, uid := range uids {
		e1, err1 := s.readyDB.Exists([]byte(uid))
		if err1 == nil && !e1 {
			e2, err2 := s.failedDB.Exists([]byte(uid))
			if err2 == nil && !e2 {
				orphaned_uids = append(orphaned_uids, uid)
			}
		}
	}

	// if no match, delete the message
	for _, uid := range orphaned_uids {
		s.messagesDB.Delete(uid)
	}
}

func (s *MessageStore) resetStuckInSent() {
	iter_opts := badger.IteratorOptions{
		PrefetchSize: 1000,
		FetchValues:  false,
		Reverse:      false,
	}

	uids := []string{}
	iter := s.sentDB.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		uids = append(uids, string(iter.Item().Key()))
	}
	iter.Close()

	s.logger.Info("Pushing back stuck messages from Sent to Ready", "nb_messages", len(uids))
	for _, uid := range uids {
		s.sentDB.Delete([]byte(uid))
		s.readyDB.Set([]byte(uid), []byte("true"))
	}

}

func (s *MessageStore) ReadAllBadgers() (map[string]string, map[string]string, map[string]string) {
	iter_opts := badger.IteratorOptions{
		FetchValues: true,
		Reverse:     false,
	}

	readyMap := map[string]string{}
	iter := s.readyDB.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		readyMap[string(item.Key())] = string(item.Value())
	}
	iter.Close()

	failedMap := map[string]string{}
	iter = s.failedDB.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		failedMap[string(item.Key())] = string(item.Value())
	}
	iter.Close()

	sentMap := map[string]string{}
	iter = s.sentDB.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		sentMap[string(item.Key())] = string(item.Value())
	}
	iter.Close()

	return readyMap, failedMap, sentMap
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
		iter := s.failedDB.NewIterator(iter_opts)
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
		iter.Close()

		if len(uids) == 0 {
			s.failed_mu.Unlock()
			break
		}

		s.ready_mu.Lock()

		deleteEntries := []*badger.Entry{}
		setEntries := []*badger.Entry{}
		for _, uid := range uids {
			s.logger.Debug("Will retry failed message", "uid", uid)
			deleteEntries = badger.EntriesDelete(deleteEntries, []byte(uid))
			setEntries = badger.EntriesSet(setEntries, []byte(uid), []byte("true"))
		}
		err := s.readyDB.BatchSet(setEntries)
		if err != nil {
			s.logger.Error("Error pushing entries from failed queue to ready queue!")
		} else {
			err := s.failedDB.BatchSet(deleteEntries)
			if err != nil {
				s.logger.Error("Error deleting entries from failed queue!")
			} else {
				s.logger.Debug("Messages pushed back from failed queue to ready queue", "nb_messages", len(uids))
			}
		}
		s.ready_mu.Unlock()
		s.failed_mu.Unlock()
	}
}

func (s *MessageStore) stash(m *model.TcpUdpParsedMessage) error {

	b, err := json.Marshal(m)
	if err != nil {
		s.logger.Warn("The store discarded a message that could not be JSON-marshalled", "error", err)
		return nil
	}
	s.ready_mu.Lock()
	defer s.ready_mu.Unlock()
	s.messages_mu.Lock()
	defer s.messages_mu.Unlock()
	err = s.messagesDB.Set(m.Uid, b)
	if err != nil {
		return err
	}
	err = s.readyDB.Set([]byte(m.Uid), []byte("true"))
	if err != nil {
		s.logger.Warn("Error putting message to Ready", "error", err)
		s.messagesDB.Delete(m.Uid)
		return err
	}
	return nil
}

func (s *MessageStore) retrieve(n int) (messages map[string]*model.TcpUdpParsedMessage) {
	s.ready_mu.Lock()
	defer s.ready_mu.Unlock()
	messages = map[string]*model.TcpUdpParsedMessage{}
	iter_opts := badger.IteratorOptions{
		PrefetchSize: n,
		FetchValues:  false,
		Reverse:      false,
	}
	s.messages_mu.Lock()
	iter := s.readyDB.NewIterator(iter_opts)
	fetched := 0
	invalidEntries := []string{}
	for iter.Rewind(); iter.Valid() && fetched < n; iter.Next() {
		uid := iter.Item().Key()
		message_b, err := s.messagesDB.Get(string(uid))
		if err == nil {
			if message_b != nil {
				message := model.TcpUdpParsedMessage{}
				err := json.Unmarshal(message_b, &message)
				if err == nil {
					messages[string(uid)] = &message
					fetched++
				} else {
					invalidEntries = append(invalidEntries, string(uid))
				}
			} else {
				invalidEntries = append(invalidEntries, string(uid))
			}
		} else {
			s.logger.Warn("Error getting message content from message queue", "uid", string(uid))
		}
	}
	iter.Close()

	for _, uid := range invalidEntries {
		s.logger.Debug("Deleting invalid entry", "uid", string(uid))
		s.readyDB.Delete([]byte(uid))
		s.messagesDB.Delete(uid)
	}
	s.messages_mu.Unlock()

	if len(messages) == 0 {
		return messages
	}
	deleteEntries := []*badger.Entry{}
	setEntries := []*badger.Entry{}
	for uid, _ := range messages {
		deleteEntries = badger.EntriesDelete(deleteEntries, []byte(uid))
		setEntries = badger.EntriesSet(setEntries, []byte(uid), []byte("true"))
	}
	err := s.sentDB.BatchSet(setEntries)
	if err != nil {
		s.logger.Error("Error pushing ready messages to the sent queue!")
		return map[string]*model.TcpUdpParsedMessage{}
	} else {
		err := s.readyDB.BatchSet(deleteEntries)
		if err != nil {
			s.logger.Error("Error deleting ready messages!")
		}
	}
	return messages
}

func (s *MessageStore) doACK(uid string) {
	err := s.sentDB.Delete([]byte(uid))
	if err != nil {
		s.logger.Warn("Error ACKing message", "uid", uid, "error", err)
	} else {
		s.messages_mu.Lock()
		err := s.messagesDB.Delete(uid)
		s.messages_mu.Unlock()
		if err != nil {
			s.logger.Warn("Error ACKing message", "uid", uid, "error", err)
		}
	}
}

func (s *MessageStore) doNACK(id string) {
	s.failed_mu.Lock()
	defer s.failed_mu.Unlock()
	s.logger.Debug("NACK", "uid", id)
	s.sentDB.Delete([]byte(id))
	s.failedDB.Set([]byte(id), []byte(time.Now().Format(time.RFC3339)))
}
