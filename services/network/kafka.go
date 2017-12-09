package network

import (
	"runtime"
	"strings"
	"sync"
	"time"

	cluster "github.com/bsm/sarama-cluster"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
	"github.com/stephane-martin/skewer/utils/queue/kafka"
)

type kafkaMetrics struct {
	ParsingErrorCounter *prometheus.CounterVec
	IncomingMsgsCounter *prometheus.CounterVec
}

func newKafkaMetrics() *kafkaMetrics {
	m := kafkaMetrics{
		IncomingMsgsCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_incoming_messages_total",
				Help: "total number of messages that were received",
			},
			[]string{"protocol", "client", "port", "path"},
		),
		ParsingErrorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_parsing_errors_total",
				Help: "total number of times there was a parsing error",
			},
			[]string{"protocol", "client", "parser_name"},
		),
	}
	return &m
}

type KafkaServiceImpl struct {
	configs          []conf.KafkaSourceConfig
	parserConfigs    []conf.ParserConfig
	reporter         *base.Reporter
	generator        chan ulid.ULID
	metrics          *kafkaMetrics
	registry         *prometheus.Registry
	rawMessagesQueue *kafka.Ring
	MaxMessageSize   int
	logger           log15.Logger
	wg               sync.WaitGroup
	stopChan         chan struct{}
	rawpool          *sync.Pool
	queues           *queue.KafkaQueues
}

func NewKafkaService(reporter *base.Reporter, gen chan ulid.ULID, l log15.Logger) *KafkaServiceImpl {
	s := KafkaServiceImpl{
		reporter:  reporter,
		generator: gen,
		metrics:   newKafkaMetrics(),
		registry:  prometheus.NewRegistry(),
		logger:    l.New("class", "KafkaService"),
		stopChan:  make(chan struct{}),
	}
	s.registry.MustRegister(s.metrics.IncomingMsgsCounter, s.metrics.ParsingErrorCounter)
	return &s
}

func (s *KafkaServiceImpl) SetConf(sc []conf.KafkaSourceConfig, pc []conf.ParserConfig, queueSize uint64) {
	s.rawpool = &sync.Pool{New: func() interface{} {
		return &model.RawKafkaMessage{}
	}}
	s.configs = sc
	s.parserConfigs = pc
	s.rawMessagesQueue = kafka.NewRing(queueSize)
}

func (s *KafkaServiceImpl) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *KafkaServiceImpl) Start(test bool) (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	s.queues = queue.NewQueueFactory()
	s.stopChan = make(chan struct{})
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
			consumer, err = config.GetClient()
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
	env := NewParsersEnv(s.parserConfigs, s.logger)
	for {
		raw, err := s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			return
		}
		s.ParseOne(env, raw)
	}
}

func (s *KafkaServiceImpl) ParseOne(env *ParsersEnv, raw *model.RawKafkaMessage) {
	// be sure to free the raw pointer
	defer s.rawpool.Put(raw)
	ackQueue := s.queues.Get(raw.ConsumerID)
	if ackQueue == nil {
		// the kafka consumer is gone
		return
	}
	// be sure to ack the message to kafka
	defer func() {
		_ = ackQueue.Put(queue.KafkaProducerAck{
			Offset: raw.Offset,
			TopicPartition: queue.TopicPartition{
				Partition: raw.Partition,
				Topic:     raw.Topic,
			},
		})
	}()

	logger := s.logger.New(
		"protocol", "kafka",
		"format", raw.Format,
		"brokers", raw.Brokers,
		"topic", raw.Topic,
	)
	decoder := utils.SelectDecoder(raw.Encoding)
	parser := env.GetParser(raw.Format)
	if parser == nil {
		logger.Error("Unknown parser")
		return
	}

	syslogMsg, err := parser.Parse(raw.Message, decoder, false)
	if err != nil {
		s.metrics.ParsingErrorCounter.WithLabelValues("kafka", raw.Brokers, raw.Format).Inc()
		logger.Info("Parsing error", "Message", raw.Message, "error", err)
		return
	}
	if syslogMsg == nil {
		return
	}

	fatal, nonfatal := s.reporter.Stash(model.FullMessage{
		Parsed: model.ParsedMessage{
			Fields: *syslogMsg,
			Client: raw.Brokers,
		},
		Uid:    raw.UID,
		ConfId: raw.ConfID,
	})

	if fatal != nil {
		logger.Error("Fatal error stashing Kafka message", "error", fatal)
		// TODO: shutdown
	} else if nonfatal != nil {
		logger.Warn("Non-fatal error stashing Kafka message", "error", nonfatal)
	}

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

	for {
		select {
		case err := <-consumer.Errors():
			if model.IsFatalKafkaError(err) {
				s.logger.Warn("Kafka consumer fatal error", "error", err)
				return
			}
			s.logger.Info("Kafka consumer non fatal error", "error", err)
		case msg := <-consumer.Messages():
			raw := s.rawpool.Get().(*model.RawKafkaMessage)
			raw.UID = <-s.generator
			raw.Brokers = brokers
			raw.ConfID = config.ConfID
			raw.ConsumerID = ackQueue.ID()
			raw.Encoding = config.Encoding
			raw.Format = config.Format
			raw.Message = msg.Value
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
