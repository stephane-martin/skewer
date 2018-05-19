package network

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	cluster "github.com/bsm/sarama-cluster"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	metrics "github.com/rcrowley/go-metrics"
	circuit "github.com/rubyist/circuitbreaker"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/decoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue"
	"github.com/stephane-martin/skewer/utils/queue/kafka"
)

var incomingByteRate *prometheus.GaugeVec

func initKafkaRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

var rawkafkapool = &sync.Pool{New: func() interface{} {
	return &model.RawKafkaMessage{
		Message: make([]byte, 0, 4096),
	}
}}

func rawKafkaFactory(data []byte) (raw *model.RawKafkaMessage) {
	raw = rawkafkapool.Get().(*model.RawKafkaMessage)
	if cap(raw.Message) < len(data) {
		raw.Message = make([]byte, 0, len(data))
	}
	raw.Message = raw.Message[:len(data)]
	copy(raw.Message, data)
	return raw
}

func freeRawKafka(raw *model.RawKafkaMessage) {
	rawkafkapool.Put(raw)
}

type KafkaServiceImpl struct {
	configs          []conf.KafkaSourceConfig
	parserConfigs    []conf.ParserConfig
	parserEnv        *decoders.ParsersEnv
	reporter         *base.Reporter
	rawMessagesQueue *kafka.Ring
	MaxMessageSize   int
	logger           log15.Logger
	wg               sync.WaitGroup
	stopCtx          context.Context
	stop             context.CancelFunc
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
		confined: env.Confined,
	}
	return &s, nil
}

func (s *KafkaServiceImpl) Type() base.Types {
	return base.KafkaSource
}

func (s *KafkaServiceImpl) SetConf(c conf.BaseConfig) {
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
	s.stopCtx, s.stop = context.WithCancel(context.Background())
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}

	// start the workers that fetch messages from the kafka cluster
	// workers push raw messages from kafka to the rawMessagesQueue
	var workersWG sync.WaitGroup
	for _, config := range s.configs {
		workersWG.Add(1)
		go func(c conf.KafkaSourceConfig) {
			defer workersWG.Done()
			s.startWorker(s.stopCtx, c)
		}(config)
	}

	// when all workers have returned, we know that we won't receive any more raw messages
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		workersWG.Wait()
		s.rawMessagesQueue.Dispose()
	}()

	// start the parsers that consume raw messages from the rawMessagesQueue
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := s.parse()
			if err != nil {
				s.logger.Error(err.Error())
				s.dofatal()
			}
		}()
	}

	return infos, nil
}

func (s *KafkaServiceImpl) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *KafkaServiceImpl) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func getConsumer(ctx context.Context, logger log15.Logger, breaker *circuit.Breaker, config conf.KafkaSourceConfig, confined bool) (*cluster.Consumer, metrics.Registry) {
	var consumer *cluster.Consumer
	var mregistry metrics.Registry

	getClient := func() (err error) {
		ret := make(chan struct{})
		go func() {
			consumer, mregistry, err = config.GetClient(confined)
			close(ret)
		}()
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-ret:
			return err
		}
	}

	for {
		// arg timeout is 0 because the sarama client handles connection timeout
		// through DialTimeout
		err := breaker.CallContext(ctx, getClient, 0)

		if err == nil {
			return consumer, mregistry
		}
		if err == context.Canceled {
			return nil, nil
		}
		if err != circuit.ErrBreakerOpen {
			logger.Debug("Error getting a Kafka consumer", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil, nil
		case <-time.After(1 * time.Second):
		}
	}
}

func (s *KafkaServiceImpl) startWorker(ctx context.Context, config conf.KafkaSourceConfig) {
	breaker := circuit.NewConsecutiveBreaker(3)

	// the for loop here makes us retry if we lose the connection to the kafka cluster
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		consumer, mregistry := getConsumer(ctx, s.logger, breaker, config, s.confined)
		if consumer == nil {
			return
		}
		s.handleConsumer(ctx, config, consumer, mregistry)
	}
}

func (s *KafkaServiceImpl) parse() (err error) {
	for {
		raw, err := s.rawMessagesQueue.Get()
		if raw == nil || err != nil {
			return nil
		}
		err = s.parseOne(raw)
		if err != nil {
			base.CountParsingError(base.KafkaSource, raw.Client, raw.Decoder.Format)
			logg(s.logger, &raw.RawMessage).Warn(err.Error())
			if eerrors.IsFatal(err) {
				freeRawKafka(raw)
				return err
			}
		}

		// ack the raw message to the kafka cluster
		ackQueue := s.queues.Get(raw.ConsumerID)
		if ackQueue != nil {
			ackQueue.Put(raw.Offset, raw.Partition, raw.Topic)
		}
		freeRawKafka(raw)
	}
}

func (s *KafkaServiceImpl) parseOne(raw *model.RawKafkaMessage) (err error) {
	syslogMsgs, err := s.parserEnv.Parse(&raw.Decoder, raw.Message)
	if err != nil {
		return err
	}

	for _, syslogMsg := range syslogMsgs {
		if syslogMsg == nil {
			continue
		}
		full := model.FullFactoryFrom(syslogMsg)
		full.Uid = raw.UID
		full.ConfId = raw.ConfID
		full.SourceType = "kafka"
		full.ClientAddr = raw.Client
		err := s.reporter.Stash(full)
		model.FullFree(full)

		if err != nil {
			logg(s.logger, &raw.RawMessage).Warn("Error stashing Kafka message", "error", err)
			if eerrors.IsFatal(err) {
				return eerrors.Wrap(err, "Fatal error pushing Kafka message to the Store")
			}
		}
	}
	return nil
}

func (s *KafkaServiceImpl) Shutdown() {
	s.Stop()
}

func (s *KafkaServiceImpl) Stop() {
	s.stop()
	// the call to s.stop() makes every worker to return eventually
	// when every worker has returned, rawMessagesQueue is disposed
	// because of that, the parsers will return too
	s.wg.Wait()
}

func (s *KafkaServiceImpl) handleConsumer(ctx context.Context, config conf.KafkaSourceConfig, consumer *cluster.Consumer, mregistry metrics.Registry) {
	if consumer == nil {
		s.logger.Error("BUG: the consumer passed to handleConsumer is NIL")
		return
	}
	var wg sync.WaitGroup
	lctx, lcancel := context.WithCancel(ctx)
	defer lcancel()
	ackQueue := s.queues.New()

	incomingByteRate := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Help: "Kafka consumers incoming byte rate",
			Name: fmt.Sprintf("skw_kafka_consumer_incoming_byte_rate_%d", ackQueue.ID()),
		},
		func() float64 {
			meter := mregistry.Get("incoming-byte-rate")
			if meter == nil {
				return 0
			}
			return meter.(metrics.Meter).Rate1()
		},
	)
	base.Registry.MustRegister(incomingByteRate)
	defer base.Registry.Unregister(incomingByteRate)

	go func() {
		<-lctx.Done()
		consumer.Close()
	}()

	wg.Add(1)
	// ack messages to kafka when needed
	// the goroutine returns eventually after the consumer has been closed
	go func() {
		defer wg.Done()
		processedMsgs := map[queue.TopicPartition](map[int64]bool){}
		nextToACK := map[queue.TopicPartition]int64{}

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

	wg.Add(1)
	// watch kafka errors
	// the goroutine returns eventually after the consumer has been closed
	go func() {
		defer wg.Done()
		for err := range consumer.Errors() {
			if model.IsFatalKafkaError(err) {
				s.logger.Warn("Kafka consumer fatal error", "error", err)
				lcancel()
			} else {
				s.logger.Info("Kafka consumer non fatal error", "error", err)
			}
		}
	}()

	wg.Add(1)
	// watch kafka messages
	// the goroutine returns eventually after the consumer has been closed
	go func() {
		defer wg.Done()
		gen := utils.NewGenerator()
		brokers := strings.Join(config.Brokers, ",")

	Loop:
		for msg := range consumer.Messages() {
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
				// if the message is rejected, immediately ACK it to Kafka
				ackQueue.Put(msg.Offset, msg.Partition, msg.Topic)
				continue Loop
			}
			raw := rawKafkaFactory(value)
			raw.UID = gen.Uid()
			raw.Client = brokers
			raw.ConfID = config.ConfID
			raw.ConsumerID = ackQueue.ID()
			raw.Decoder = config.DecoderBaseConfig
			raw.Topic = msg.Topic
			raw.Partition = msg.Partition
			raw.Offset = msg.Offset
			s.rawMessagesQueue.Put(raw)
			base.CountIncomingMessage(base.KafkaSource, raw.Client, 0, "")
		}

		// the previous for loop returns when the Messages channel has been closed
		// the Messages channel is only closed after the consumer has been closed
		// so here we now that the current kafka consumer is gone
		// hence, there is no need to process ACK any further
		s.queues.Delete(ackQueue)

	}()

	wg.Wait()
}
