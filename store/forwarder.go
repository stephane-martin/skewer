package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	sarama "gopkg.in/Shopify/sarama.v1"
)

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

// TODO: merge kafkaForwarder and dummyKafkaForwarder

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
	processingErrorsChan := from.ProcessingErrors()
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
					fwder.logger.Warn("Could not find the stored configuration for a message", "confId", message.ConfId, "msgId", message.Uid)
					processingErrorsChan <- message.Uid
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
				processingErrorsChan <- message.Uid
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
				processingErrorsChan <- message.Uid
				fwder.logger.Warn("Error happened processing message", "uid", message.Uid, "error", err)
				fwder.metrics.MessageFilteringCounter.WithLabelValues("unknown", message.Parsed.Client).Inc()
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
	processingErrorsChan := from.ProcessingErrors()
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
					fwder.logger.Warn("Could not find the stored configuration for a message", "confId", message.ConfId, "msgId", message.Uid)
					processingErrorsChan <- message.Uid
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
					processingErrorsChan <- message.Uid
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
					processingErrorsChan <- message.Uid
					fwder.logger.Warn("Error happened when processing message", "uid", message.Uid, "error", err)
					fwder.metrics.MessageFilteringCounter.WithLabelValues("unknown", message.Parsed.Client).Inc()
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

				fwder.logger.Info("Message", "partitionkey", string(pkey), "topic", kafkaMsg.Topic, "msgid", message.Uid)
				fmt.Println(string(v))
				fmt.Println()

				ackChan <- message.Uid
			}
		}
	}
}
