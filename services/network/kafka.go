package network

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	cluster "github.com/bsm/sarama-cluster"
	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
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
	stopCtx          context.Context
	stop             context.CancelFunc
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
	s.stopCtx, s.stop = context.WithCancel(context.Background())
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}

	// start the workers that fetch messages from the kafka cluster
	// workers push raw messages from kafka to the rawMessagesQueue
	var workersWG sync.WaitGroup
	for _, config := range s.configs {
		workersWG.Add(1)
		go func() {
			defer workersWG.Done()
			s.startWorker(s.stopCtx, config)
		}()
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

func (s *KafkaServiceImpl) startWorker(ctx context.Context, config conf.KafkaSourceConfig) {
	breaker := circuit.NewConsecutiveBreaker(3)

	getConsumer := func() (consumer *cluster.Consumer) {

		getClient := func() (err error) {
			consumer, err = config.GetClient(s.confined)
			return err
		}

		for {
			// arg timeout is 0 because the sarama client handles connection timeout
			// through DialTimeout
			err := breaker.CallContext(ctx, getClient, 0)

			if err == nil {
				return consumer
			}
			if err == context.Canceled {
				return nil
			}
			if err != circuit.ErrBreakerOpen {
				s.logger.Debug("Error getting a Kafka consumer", "error", err)
			}

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(1 * time.Second):
			}
		}
	}

	// the for loop here makes us retry if we lose a connection to the kafka cluster
	for {
		consumer := getConsumer()
		if consumer == nil {
			return
		}
		s.handleConsumer(ctx, config, consumer)
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
			base.ParsingErrorCounter.WithLabelValues("kafka", raw.Client, raw.Decoder.Format).Inc()
			logg(s.logger, &raw.RawMessage).Warn(err.Error())
			if eerrors.IsFatal(err) {
				s.rawpool.Put(raw)
				return err
			}
		}

		// ack the raw message to the kafka cluster
		ackQueue := s.queues.Get(raw.ConsumerID)
		if ackQueue != nil {
			_ = ackQueue.Put(queue.KafkaProducerAck{
				Offset: raw.Offset,
				TopicPartition: queue.TopicPartition{
					Partition: raw.Partition,
					Topic:     raw.Topic,
				},
			})
		}
		s.rawpool.Put(raw)
	}
}

func (s *KafkaServiceImpl) parseOne(raw *model.RawKafkaMessage) (err error) {
	parser, err := s.parserEnv.GetParser(&raw.Decoder)
	if parser == nil || err != nil {
		return decoders.DecodingError(eerrors.Wrapf(err, "Unknown decoder: %s", raw.Decoder.Format))
	}
	defer parser.Release()

	syslogMsgs, err := parser.Parse(raw.Message)
	if err != nil {
		return decoders.DecodingError(eerrors.Wrap(err, "Parsing error"))
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

func (s *KafkaServiceImpl) handleConsumer(ctx context.Context, config conf.KafkaSourceConfig, consumer *cluster.Consumer) {
	if consumer == nil {
		s.logger.Error("BUG: the consumer passed to handleConsumer is NIL")
		return
	}
	var wg sync.WaitGroup
	lctx, lcancel := context.WithCancel(ctx)
	ackQueue := s.queues.New()

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
				consumer.Close()
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
			raw.Client = brokers
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
			} else {
				base.IncomingMsgsCounter.WithLabelValues("kafka", raw.Client, "", "").Inc()
			}
		}

		// the previous for loop returns when the Messages channel has been closed
		// the Messages channel is only closed after the consumer has been closed
		// so here we now that the current kafka consumer is gone
		// hence, there is no need to process ACK any further
		s.queues.Delete(ackQueue)

	}()

	wg.Wait()
	lcancel()
}
