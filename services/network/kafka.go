package network

import (
	"bytes"
	"runtime"
	"strings"
	"sync"
	"time"

	cluster "github.com/bsm/sarama-cluster"
	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
	"github.com/stephane-martin/skewer/utils/queue/kafka"
)

func initKafkaRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type KafkaServiceImpl struct {
	configs          []conf.KafkaSourceConfig
	parserConfigs    []conf.ParserConfig
	parserEnv        *decoders.ParsersEnv
	reporter         base.Stasher
	rawMessagesQueue *kafka.Ring
	MaxMessageSize   int
	logger           log15.Logger
	wg               sync.WaitGroup
	stopChan         chan struct{}
	rawpool          *sync.Pool
	queues           *queue.KafkaQueues
	fatalErrorChan   chan struct{}
	fatalOnce        *sync.Once
	confined         bool
}

func NewKafkaService(env *base.ProviderEnv) (base.Provider, error) {
	initKafkaRegistry()
	s := KafkaServiceImpl{
		reporter: env.Reporter,
		logger:   env.Logger.New("class", "KafkaService"),
		stopChan: make(chan struct{}),
		confined: env.Confined,
	}
	return &s, nil
}

func (s *KafkaServiceImpl) Type() base.Types {
	return base.KafkaSource
}

func (s *KafkaServiceImpl) SetConf(c conf.BaseConfig) {
	s.rawpool = &sync.Pool{New: func() interface{} {
		return &model.RawKafkaMessage{}
	}}
	s.configs = c.KafkaSource
	s.parserConfigs = c.Parsers
	s.parserEnv = decoders.NewParsersEnv(s.parserConfigs, s.logger)
	s.rawMessagesQueue = kafka.NewRing(c.Main.InputQueueSize)
}

func (s *KafkaServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *KafkaServiceImpl) Start() (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	s.queues = queue.NewQueueFactory()
	s.stopChan = make(chan struct{})
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}
	for _, config := range s.configs {
		s.wg.Add(1)
		go s.startOne(config)
	}
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.wg.Add(1)
		go s.Parse()
	}

	return infos, nil
}

func (s *KafkaServiceImpl) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *KafkaServiceImpl) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *KafkaServiceImpl) startOne(config conf.KafkaSourceConfig) {
	defer s.wg.Done()
	var consumer *cluster.Consumer
	var err error
	for {
		for {
			select {
			case <-s.stopChan:
				return
			default:
			}
			consumer, err = config.GetClient(s.confined)
			if err == nil {
				s.logger.Debug("Got a Kafka consumer")
				break
			}
			s.logger.Debug("Error getting a Kafka consumer", "error", err)
			select {
			case <-s.stopChan:
				return
			case <-time.After(2 * time.Second):
			}
		}
		s.handleConsumer(config, consumer)
	}
}

func (s *KafkaServiceImpl) Parse() {
	defer s.wg.Done()
	var err error
	var raw *model.RawKafkaMessage

	for {
		raw, err = s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			return
		}
		err = s.ParseOne(raw)
		s.rawpool.Put(raw)
		if err != nil {
			return
		}
	}
}

func (s *KafkaServiceImpl) ParseOne(raw *model.RawKafkaMessage) (err error) {
	ackQueue := s.queues.Get(raw.ConsumerID)
	if ackQueue == nil {
		// the kafka consumer is gone
		return nil
	}
	// be sure to ack the message to kafka
	defer func() {
		if err == nil {
			_ = ackQueue.Put(queue.KafkaProducerAck{
				Offset: raw.Offset,
				TopicPartition: queue.TopicPartition{
					Partition: raw.Partition,
					Topic:     raw.Topic,
				},
			})
		}
	}()

	logger := s.logger.New(
		"protocol", "kafka",
		"format", raw.Decoder.Format,
		"brokers", raw.Brokers,
		"topic", raw.Topic,
	)
	parser, err := s.parserEnv.GetParser(&raw.Decoder)
	if parser == nil || err != nil {
		logger.Error("Unknown parser")
		return nil
	}
	defer parser.Release()

	syslogMsgs, err := parser.Parse(raw.Message)
	if err != nil {
		base.ParsingErrorCounter.WithLabelValues("kafka", raw.Brokers, raw.Decoder.Format).Inc()
		logger.Info("Parsing error", "error", err)
		return nil
	}
	var syslogMsg *model.SyslogMessage
	var full *model.FullMessage
	for _, syslogMsg = range syslogMsgs {
		if syslogMsg == nil {
			continue
		}
		if raw.Brokers != "" {
			syslogMsg.SetProperty("skewer", "client", raw.Brokers)
		}
		full = model.FullFactoryFrom(syslogMsg)
		full.Uid = raw.UID
		full.ConfId = raw.ConfID
		fatal, nonfatal := s.reporter.Stash(full)

		if fatal != nil {
			err = fatal
			logger.Error("Fatal error stashing Kafka message", "error", fatal)
			s.dofatal()
			return err
		} else if nonfatal != nil {
			logger.Warn("Non-fatal error stashing Kafka message", "error", nonfatal)
		} else {
			base.IncomingMsgsCounter.WithLabelValues("kafka", raw.Brokers, "", "").Inc()
		}
		model.FullFree(full)
	}
	return nil
}

func (s *KafkaServiceImpl) Shutdown() {
	s.Stop()
}

func (s *KafkaServiceImpl) Stop() {
	close(s.stopChan)
	s.rawMessagesQueue.Dispose()
	s.wg.Wait()
}

func (s *KafkaServiceImpl) handleConsumer(config conf.KafkaSourceConfig, consumer *cluster.Consumer) {
	brokers := strings.Join(config.Brokers, ",")
	ackQueue := s.queues.New()
	defer s.queues.Delete(ackQueue)

	nextToACK := map[queue.TopicPartition]int64{}

	go func() {
		defer func() {
			_ = consumer.Close()
		}()
		processedMsgs := map[queue.TopicPartition](map[int64]bool){}

		for ackQueue.Wait() {
			ack, err := ackQueue.Get()
			if err != nil {
				return
			}
			if len(ack.Topic) == 0 {
				continue
			}
			// a little dance to ACK kafka messages in growing order for each partition
			if _, ok := processedMsgs[ack.TopicPartition]; !ok {
				processedMsgs[ack.TopicPartition] = map[int64]bool{}
			}
			processedMsgs[ack.TopicPartition][ack.Offset] = true
			next, ok := nextToACK[ack.TopicPartition]
			if !ok {
				next = ack.Offset
			}
			for processedMsgs[ack.TopicPartition][next] {
				delete(processedMsgs[ack.TopicPartition], next)
				consumer.MarkPartitionOffset(ack.Topic, ack.Partition, next, "")
				next++
				nextToACK[ack.TopicPartition] = next
			}
		}
	}()

	gen := utils.NewGenerator()

Loop:
	for {
		select {
		case err := <-consumer.Errors():
			if model.IsFatalKafkaError(err) {
				s.logger.Warn("Kafka consumer error", "error", err)
				return
			}
			s.logger.Info("Kafka consumer non fatal error", "error", err)
		case msg := <-consumer.Messages():
			ok := true
			value := bytes.TrimSpace(msg.Value)
			if len(value) == 0 {
				s.logger.Warn("Empty message")
				ok = false
			}
			if s.MaxMessageSize > 0 && len(value) > s.MaxMessageSize {
				s.logger.Warn("Message too large")
				ok = false
			}
			if !ok {
				ackQueue.Put(
					queue.KafkaProducerAck{
						Offset: msg.Offset,
						TopicPartition: queue.TopicPartition{
							Partition: msg.Partition,
							Topic:     msg.Topic,
						},
					},
				)
				continue Loop
			}
			raw := s.rawpool.Get().(*model.RawKafkaMessage)
			raw.UID = gen.Uid()
			raw.Brokers = brokers
			raw.ConfID = config.ConfID
			raw.ConsumerID = ackQueue.ID()
			raw.Decoder = config.DecoderBaseConfig
			raw.Message = raw.Message[:len(value)]
			copy(raw.Message, value)
			raw.Topic = msg.Topic
			raw.Partition = msg.Partition
			raw.Offset = msg.Offset
			err := s.rawMessagesQueue.Put(raw)
			if err != nil {
				// rawMessagesQueue has been disposed
				s.rawpool.Put(raw)
				s.logger.Warn("Error queueing kafka message", "error", err)
				return
			}
		case <-s.stopChan:
			return
		}
	}
}
