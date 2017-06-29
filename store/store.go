package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	sarama "gopkg.in/Shopify/sarama.v1"

	"github.com/dgraph-io/badger/badger"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/javascript"
	"github.com/stephane-martin/relp2kafka/model"
)

type MessageStore struct {
	test           bool
	messages       *badger.KV
	ready          *badger.KV
	sent           *badger.KV
	failed         *badger.KV
	sendStoppedMu  *sync.Mutex
	sendStopped    bool
	StopSendChan   chan bool
	ClosingChan    chan bool
	FatalErrorChan chan bool
	KafkaErrorChan chan bool
	ready_mu       *sync.Mutex
	failed_mu      *sync.Mutex
	messages_mu    *sync.Mutex
	Inputs         chan *model.TcpUdpParsedMessage
	Outputs        chan *model.TcpUdpParsedMessage
	wg             *sync.WaitGroup
	sendWg         *sync.WaitGroup
	storeToKafkaWg *sync.WaitGroup
	ticker         *time.Ticker
	logger         log15.Logger
	Conf           conf.GConfig
}

func (s *MessageStore) init() {
	s.sendStoppedMu = &sync.Mutex{}
	s.ready_mu = &sync.Mutex{}
	s.failed_mu = &sync.Mutex{}
	s.messages_mu = &sync.Mutex{}
	s.wg = &sync.WaitGroup{}
	s.sendWg = &sync.WaitGroup{}
	s.storeToKafkaWg = &sync.WaitGroup{}
	s.ClosingChan = make(chan bool)
	s.StopSendChan = make(chan bool)
	s.FatalErrorChan = make(chan bool)
	s.KafkaErrorChan = make(chan bool)
}

func (s *MessageStore) NewJsEnvs() map[int]*javascript.Environment {
	jsenvs := map[int]*javascript.Environment{}
	for i, syslogConf := range s.Conf.Syslog {
		if syslogConf.Protocol != "relp" {
			jsenvs[i] = javascript.New(
				syslogConf.FilterFunc,
				syslogConf.TopicFunc,
				syslogConf.TopicTemplate,
				syslogConf.PartitionFunc,
				syslogConf.PartitionKeyTemplate,
				s.logger,
			)
		}
	}
	return jsenvs
}

func (s *MessageStore) SetNewConf(newConf *conf.GConfig) {
	s.Conf = *newConf
}

func NewStore(c *conf.GConfig, l log15.Logger, test bool) (store *MessageStore, err error) {
	dirname := c.Store.Dirname
	opts_messages := badger.DefaultOptions
	opts_ready := badger.DefaultOptions
	opts_sent := badger.DefaultOptions
	opts_failed := badger.DefaultOptions
	opts_messages.Dir = path.Join(dirname, "messages")
	opts_messages.ValueDir = path.Join(dirname, "messages")
	opts_sent.Dir = path.Join(dirname, "sent")
	opts_sent.ValueDir = path.Join(dirname, "sent")
	opts_ready.Dir = path.Join(dirname, "ready")
	opts_ready.ValueDir = path.Join(dirname, "ready")
	opts_failed.Dir = path.Join(dirname, "failed")
	opts_failed.ValueDir = path.Join(dirname, "failed")
	opts_messages.MaxTableSize = c.Store.Maxsize
	opts_messages.SyncWrites = c.Store.FSync
	opts_ready.SyncWrites = c.Store.FSync
	opts_sent.SyncWrites = c.Store.FSync
	opts_failed.SyncWrites = c.Store.FSync

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

	store = &MessageStore{}
	store.logger = l.New("class", "MessageStore")
	store.init()
	store.SetNewConf(c)
	store.test = test

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
	store.sendStopped = true

	// only once, push back messages from previous run that may have been stuck in the sent queue
	store.resetStuckInSent()

	// prune orphaned messages
	store.pruneOrphaned()

	store.ticker = time.NewTicker(time.Minute)
	store.wg.Add(1)
	go func() {
		defer store.wg.Done()
		defer store.logger.Debug("End of periodic Store.resetFailures")
		for {
			select {
			case <-store.ticker.C:
				store.resetFailures()
			case <-store.ClosingChan:
				return
			}

		}
	}()
	store.startIngest()

	return store, nil
}

func (s *MessageStore) SendToKafka() {
	s.KafkaErrorChan = make(chan bool)
	s.storeToKafkaWg.Add(1)
	go s.store2kafka()
}

func (s *MessageStore) StopSendToKafka() {
	s.logger.Debug("Store.StopSend called")
	s.sendStoppedMu.Lock()
	if s.sendStopped {
		s.sendStoppedMu.Unlock()
		return
	}
	s.sendStopped = true
	close(s.StopSendChan)
	s.sendStoppedMu.Unlock()
	s.logger.Debug("Store.StopSend waiting for StartSend to finish")
	s.sendWg.Wait()         // wait that StartSend has finished
	s.storeToKafkaWg.Wait() // wait that Store2Kafka has finished
	s.logger.Debug("Store.StopSend finished")
}

func (s *MessageStore) Close() {
	s.StopSendToKafka()
	close(s.ClosingChan) // causes resetFailures to end
	close(s.Inputs)      // causes ingest to end
	s.ticker.Stop()      // stop to trigger resetFailures
	s.wg.Wait()          // wait that ingest and resetFailures have finished
	s.CloseBadgerDB()    // close the badger databases

}

func (s *MessageStore) CloseBadgerDB() {
	err := s.messages.Close()
	if err != nil {
		s.logger.Warn("Error closing Messages in store", "error", err)
	}
	err = s.ready.Close()
	if err != nil {
		s.logger.Warn("Error closing Ready in store", "error", err)
	}
	err = s.sent.Close()
	if err != nil {
		s.logger.Warn("Error closing Sent in store", "error", err)
	}
	err = s.failed.Close()
	if err != nil {
		s.logger.Warn("Error closing Failed in store", "error", err)
	}
	s.logger.Info("Badger databases are closed")
}

func (s *MessageStore) SendStopped() bool {
	s.sendStoppedMu.Lock()
	defer s.sendStoppedMu.Unlock()
	return s.sendStopped
}

func (s *MessageStore) pruneOrphaned() {
	// find if we have some old full messages
	iter_opts := badger.IteratorOptions{
		PrefetchSize: 1000,
		FetchValues:  false,
		Reverse:      false,
	}

	uids := []string{}
	iter := s.messages.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		uids = append(uids, string(iter.Item().Key()))
	}
	iter.Close()

	// check if the corresponding uid exists in "ready" or "failed"
	orphaned_uids := []string{}
	for _, uid := range uids {
		e1, err1 := s.ready.Exists([]byte(uid))
		if err1 == nil && !e1 {
			e2, err2 := s.failed.Exists([]byte(uid))
			if err2 == nil && !e2 {
				orphaned_uids = append(orphaned_uids, uid)
			}
		}
	}

	// if no match, delete the message
	for _, uid := range orphaned_uids {
		s.messages.Delete([]byte(uid))
	}
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
		uids = append(uids, string(iter.Item().Key()))
	}
	iter.Close()

	s.logger.Info("Pushing back stuck messages from Sent to Ready", "nb_messages", len(uids))
	for _, uid := range uids {
		s.sent.Delete([]byte(uid))
		s.ready.Set([]byte(uid), []byte("true"))
	}

}

func (s *MessageStore) ReadAll() (map[string]string, map[string]string, map[string]string, map[string]string) {
	iter_opts := badger.IteratorOptions{
		FetchValues: true,
		Reverse:     false,
	}

	messagesMap := map[string]string{}
	iter := s.messages.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		messagesMap[string(item.Key())] = string(item.Value())
	}
	iter.Close()

	readyMap := map[string]string{}
	iter = s.ready.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		readyMap[string(item.Key())] = string(item.Value())
	}
	iter.Close()

	failedMap := map[string]string{}
	iter = s.failed.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		failedMap[string(item.Key())] = string(item.Value())
	}
	iter.Close()

	sentMap := map[string]string{}
	iter = s.sent.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		item := iter.Item()
		sentMap[string(item.Key())] = string(item.Value())
	}
	iter.Close()

	return messagesMap, readyMap, failedMap, sentMap
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
		err := s.ready.BatchSet(setEntries)
		if err != nil {
			s.logger.Error("Error pushing entries from failed queue to ready queue!")
		} else {
			err := s.failed.BatchSet(deleteEntries)
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

func (s *MessageStore) startIngest() {
	s.logger.Debug("startIngest")
	s.Inputs = make(chan *model.TcpUdpParsedMessage, 10000)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		var err error
		for m := range s.Inputs {
			err = s.stash(m)
			if err != nil {
				if err == badger.ErrNoRoom {
					s.logger.Crit("The store is full!")
					close(s.FatalErrorChan) // signal the server than we should stop everything
					return
				} else {
					s.logger.Warn("Store unexpected error", "error", err)
				}
			}
		}
		s.logger.Debug("ingestion goroutine has finished")
	}()
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
	err = s.messages.Set([]byte(m.Uid), b)
	if err != nil {
		return err
	}
	err = s.ready.Set([]byte(m.Uid), []byte("true"))
	if err != nil {
		s.logger.Warn("Error putting message to Ready", "error", err)
		s.messages.Delete([]byte(m.Uid))
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
	iter := s.ready.NewIterator(iter_opts)
	fetched := 0
	invalidEntries := []string{}
	for iter.Rewind(); iter.Valid() && fetched < n; iter.Next() {
		uid := iter.Item().Key()
		item := badger.KVItem{}
		err := s.messages.Get(uid, &item)
		if err == nil {
			message_b := item.Value()
			if message_b != nil {
				message := model.TcpUdpParsedMessage{}
				err := json.Unmarshal(message_b, &message)
				if err == nil {
					messages[string(uid)] = &message
					fetched++
				} else {
					//s.logger.Warn("Error unmarshaling message from the badger", "uid", string(uid))
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
		s.ready.Delete([]byte(uid))
		s.messages.Delete([]byte(uid))
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
	err := s.sent.BatchSet(setEntries)
	if err != nil {
		s.logger.Error("Error pushing ready messages to the sent queue!")
		return map[string]*model.TcpUdpParsedMessage{}
	} else {
		err := s.ready.BatchSet(deleteEntries)
		if err != nil {
			s.logger.Error("Error deleting ready messages!")
		}
	}
	return messages
}

func (s *MessageStore) Ack(id string) {
	s.logger.Debug("ACK", "uid", id)
	err := s.sent.Delete([]byte(id))
	if err != nil {
		s.logger.Warn("Error ACKing message", "uid", id, "error", err)
	} else {
		s.messages_mu.Lock()
		err := s.messages.Delete([]byte(id))
		s.messages_mu.Unlock()
		if err != nil {
			s.logger.Warn("Error ACKing message", "uid", id, "error", err)
		}
	}
}

func (s *MessageStore) Nack(id string) {
	s.failed_mu.Lock()
	defer s.failed_mu.Unlock()
	s.logger.Debug("NACK", "uid", id)
	s.sent.Delete([]byte(id))
	s.failed.Set([]byte(id), []byte(time.Now().Format(time.RFC3339)))
}

func (s *MessageStore) store2kafka() {
	defer func() {
		s.logger.Debug("Store2Kafka has ended")
		s.storeToKafkaWg.Done()
	}()
	s.logger.Debug("Store2Kafka")
	jsenvs := s.NewJsEnvs()
	if s.test {
		s.startSend()
	ForOutputsTest:
		for message := range s.Outputs {
			if message != nil {
				partitionKey := jsenvs[message.ConfIndex].PartitionKey(message.Parsed.Fields)
				topic := jsenvs[message.ConfIndex].Topic(message.Parsed.Fields)

				if len(topic) == 0 || len(partitionKey) == 0 {
					s.logger.Warn("Topic or PartitionKey could not be calculated", "uid", message.Uid)
					s.Nack(message.Uid)
					continue ForOutputsTest
				}

				// todo: catch err
				tmsg, filterResult, _ := jsenvs[message.ConfIndex].FilterMessage(message.Parsed.Fields)

				switch filterResult {
				case javascript.DROPPED:
					s.Ack(message.Uid)
					continue ForOutputsTest
				case javascript.REJECTED:
					s.Nack(message.Uid)
					continue ForOutputsTest
				case javascript.PASS:
					if tmsg == nil {
						s.Ack(message.Uid)
						continue ForOutputsTest
					}
				default:
					s.Nack(message.Uid)
					// todo: log the faulty message to a specific log
					continue ForOutputsTest
				}

				nmsg := model.ParsedMessage{
					Fields:         tmsg,
					Client:         message.Parsed.Client,
					LocalPort:      message.Parsed.LocalPort,
					UnixSocketPath: message.Parsed.UnixSocketPath,
				}
				kafkaMsg, err := nmsg.ToKafkaMessage(partitionKey, topic)
				if err != nil {
					s.logger.Warn("Error generating Kafka message", "error", err, "uid", message.Uid)
					s.Nack(message.Uid)
					continue ForOutputsTest
				}

				v, _ := kafkaMsg.Value.Encode()
				pkey, _ := kafkaMsg.Key.Encode()
				fmt.Printf("pkey: '%s' topic:'%s' uid:'%s'\n", pkey, kafkaMsg.Topic, message.Uid)
				fmt.Println(string(v))
				fmt.Println()

				s.Ack(message.Uid)
			}
		}
	} else {
		var producer sarama.AsyncProducer
		var err error
		for {
			producer, err = s.Conf.GetKafkaAsyncProducer()
			if err == nil {
				s.logger.Debug("Got a Kafka producer")
				break
			} else {
				s.logger.Warn("Error getting a Kafka client", "error", err)
				select {
				case <-s.StopSendChan:
					return
				case <-time.After(2 * time.Second):
				}
			}
		}
		defer producer.AsyncClose()

		// listen for kafka NACK responses
		s.storeToKafkaWg.Add(1)
		go func() {
			defer func() {
				s.storeToKafkaWg.Done()
			}()
			more_succs := true
			more_fails := true
			var succ *sarama.ProducerMessage
			var fail *sarama.ProducerError
			for more_succs || more_fails {
				select {
				case succ, more_succs = <-producer.Successes():
					if more_succs {
						uid := succ.Metadata.(string)
						s.Ack(uid)
					}

				case fail, more_fails = <-producer.Errors():
					if more_fails {
						uid := fail.Msg.Metadata.(string)
						s.Nack(uid)
						s.logger.Info("Kafka producer error", "error", fail.Error())
						if model.IsFatalKafkaError(fail.Err) {
							close(s.KafkaErrorChan)
						}
					}
				}
			}
		}()

		s.startSend()
	ForOutputs:
		for message := range s.Outputs {
			partitionKey := jsenvs[message.ConfIndex].PartitionKey(message.Parsed.Fields)
			topic := jsenvs[message.ConfIndex].Topic(message.Parsed.Fields)

			if len(topic) == 0 || len(partitionKey) == 0 {
				s.logger.Warn("Topic or PartitionKey could not be calculated", "uid", message.Uid)
				s.Nack(message.Uid)
				continue ForOutputs
			}

			// todo: catch err
			tmsg, filterResult, _ := jsenvs[message.ConfIndex].FilterMessage(message.Parsed.Fields)

			switch filterResult {
			case javascript.DROPPED:
				s.Ack(message.Uid)
				continue ForOutputs
			case javascript.REJECTED:
				s.Nack(message.Uid)
				continue ForOutputs
			case javascript.PASS:
				if tmsg == nil {
					s.Ack(message.Uid)
					continue ForOutputs
				}
			default:
				s.Nack(message.Uid)
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
				s.logger.Warn("Error generating Kafka message", "error", err, "uid", message.Uid)
				s.Nack(message.Uid)
				continue ForOutputs
			}

			kafkaMsg.Metadata = message.Uid
			producer.Input() <- kafkaMsg
		}
	}
}

func (s *MessageStore) startSend() {
	s.logger.Debug("Store.startSend called")
	s.sendStoppedMu.Lock()
	if !s.sendStopped {
		s.sendStoppedMu.Unlock()
		s.logger.Debug("Store is already sending messages")
		return
	}
	s.StopSendChan = make(chan bool)
	s.Outputs = make(chan *model.TcpUdpParsedMessage)
	s.sendStopped = false
	s.sendStoppedMu.Unlock()
	s.sendWg.Add(1)
	go func() {
		s.logger.Debug("StartSend main goroutine")
		defer func() {
			s.logger.Debug("Store Send goroutine has ended")
			close(s.Outputs)
			s.sendWg.Done()
		}()
		for !s.SendStopped() {
			messages := s.retrieve(1000)
			if len(messages) == 0 {
				select {
				case <-time.After(1000 * time.Millisecond):
				case <-s.StopSendChan:
					return
				}
			} else {
				s.logger.Debug("Store has some messages to provide", "nb", len(messages))
				for _, m := range messages {
					s.Outputs <- m
				}
			}
		}
	}()
}
