package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/javascript"
	"github.com/stephane-martin/skewer/model"
	sarama "gopkg.in/Shopify/sarama.v1"
)

type forwarderMetrics struct {
	KafkaConnectionErrorCounter prometheus.Counter
	KafkaAckNackCounter         *prometheus.CounterVec
	MessageFilteringCounter     *prometheus.CounterVec
}

func NewForwarderMetrics() *forwarderMetrics {
	m := &forwarderMetrics{}
	m.KafkaConnectionErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "skw_fwder_kafka_connection_errors_total",
			Help: "number of kafka connection errors",
		},
	)

	m.KafkaAckNackCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_fwder_kafka_ack_total",
			Help: "number of kafka acknowledgments",
		},
		[]string{"status", "topic"},
	)

	m.MessageFilteringCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_fwder_messages_filtering_total",
			Help: "number of filtered messages by status",
		},
		[]string{"status", "client"},
	)
	return m
}

type kafkaForwarder struct {
	logger     log15.Logger
	errorChan  chan struct{}
	wg         *sync.WaitGroup
	forwarding int32
	test       bool
	metrics    *forwarderMetrics
	registry   *prometheus.Registry
}

func NewForwarder(test bool, logger log15.Logger) (fwder Forwarder) {
	f := kafkaForwarder{
		test:     test,
		logger:   logger.New("class", "kafkaForwarder"),
		metrics:  NewForwarderMetrics(),
		registry: prometheus.NewRegistry(),
	}
	f.registry.MustRegister(
		f.metrics.KafkaAckNackCounter,
		f.metrics.KafkaConnectionErrorCounter,
		f.metrics.MessageFilteringCounter,
	)
	f.errorChan = make(chan struct{})
	f.wg = &sync.WaitGroup{}
	return &f
}

func (fwder *kafkaForwarder) Gather() ([]*dto.MetricFamily, error) {
	return fwder.registry.Gather()
}

func (fwder *kafkaForwarder) ErrorChan() chan struct{} {
	return fwder.errorChan
}

func (fwder *kafkaForwarder) WaitFinished() {
	fwder.wg.Wait()
}

func (fwder *kafkaForwarder) Forward(ctx context.Context, from Store, to conf.KafkaConfig) {
	fwder.errorChan = make(chan struct{})
	fwder.wg.Add(1)
	go fwder.doForward(ctx, from, to)
}

func (fwder *kafkaForwarder) doForward(ctx context.Context, from Store, to conf.KafkaConfig) {
	defer fwder.wg.Done()
	var succChan <-chan *sarama.ProducerMessage
	var failChan <-chan *sarama.ProducerError
	var producer sarama.AsyncProducer

	if !fwder.test {
		producer = fwder.getProducer(ctx, &to)
		if producer == nil {
			return
		}
		succChan = producer.Successes()
		failChan = producer.Errors()
		// listen for kafka responses
		fwder.wg.Add(1)
		go fwder.listenKafkaResponses(from, succChan, failChan)
	}

	fwder.wg.Add(1)
	go fwder.getAndSendMessages(ctx, from, producer)

}

func (fwder *kafkaForwarder) getAndSendMessages(ctx context.Context, from Store, producer sarama.AsyncProducer) {
	defer func() {
		if producer != nil {
			producer.AsyncClose()
		}
		fwder.wg.Done()
	}()

	jsenvs := map[string]*javascript.Environment{}
	done := ctx.Done()
	var more bool
	var message *model.TcpUdpParsedMessage
	var topic string
	var partitionKey string
	var partitionNumber int32
	var errs []error
	var err error
	var filterResult javascript.FilterResult
	var serialized []byte
	var kafkaMsg *sarama.ProducerMessage
	var reported time.Time

ForOutputs:
	for {
		select {
		case <-done:
			return
		case message, more = <-from.Outputs():
			if !more {
				return
			}
			env, ok := jsenvs[message.ConfId]
			if !ok {
				config, err := from.GetSyslogConfig(message.ConfId)
				if err != nil {
					fwder.logger.Warn("Could not find the stored configuration for a message", "confId", message.ConfId, "msgId", message.Uid)
					from.PermError(message.Uid)
					continue ForOutputs
				}
				jsenvs[message.ConfId] = javascript.NewFilterEnvironment(
					config.FilterFunc,
					config.TopicFunc,
					config.TopicTmpl,
					config.PartitionFunc,
					config.PartitionTmpl,
					config.PartitionNumberFunc,
					fwder.logger,
				)
				env = jsenvs[message.ConfId]
			}

			topic, errs = env.Topic(message.Parsed.Fields)
			for _, err = range errs {
				fwder.logger.Info("Error calculating topic", "error", err, "uid", message.Uid)
			}
			if len(topic) == 0 {
				fwder.logger.Warn("Topic or PartitionKey could not be calculated", "uid", message.Uid)
				from.PermError(message.Uid)
				continue ForOutputs
			}
			partitionKey, errs = env.PartitionKey(message.Parsed.Fields)
			for _, err := range errs {
				fwder.logger.Info("Error calculating the partition key", "error", err, "uid", message.Uid)
			}
			partitionNumber, errs = env.PartitionNumber(message.Parsed.Fields)
			for _, err := range errs {
				fwder.logger.Info("Error calculating the partition number", "error", err, "uid", message.Uid)
			}

			filterResult, err = env.FilterMessage(&message.Parsed.Fields)

			switch filterResult {
			case javascript.DROPPED:
				from.ACK(message.Uid)
				fwder.metrics.MessageFilteringCounter.WithLabelValues("dropped", message.Parsed.Client).Inc()
				continue ForOutputs
			case javascript.REJECTED:
				fwder.metrics.MessageFilteringCounter.WithLabelValues("rejected", message.Parsed.Client).Inc()
				from.NACK(message.Uid)
				continue ForOutputs
			case javascript.PASS:
				fwder.metrics.MessageFilteringCounter.WithLabelValues("passing", message.Parsed.Client).Inc()
			default:
				from.PermError(message.Uid)
				fwder.logger.Warn("Error happened processing message", "uid", message.Uid, "error", err)
				fwder.metrics.MessageFilteringCounter.WithLabelValues("unknown", message.Parsed.Client).Inc()
				continue ForOutputs
			}

			reported = time.Unix(0, message.Parsed.Fields.TimeReportedNum).UTC()
			message.Parsed.Fields.TimeGenerated = time.Unix(0, message.Parsed.Fields.TimeGeneratedNum).UTC().Format(time.RFC3339Nano)
			message.Parsed.Fields.TimeReported = reported.Format(time.RFC3339Nano)

			serialized, err = ffjson.Marshal(&message.Parsed)

			if err != nil {
				fwder.logger.Warn("Error serializing message", "error", err, "uid", message.Uid)
				from.PermError(message.Uid)
				continue ForOutputs
			}

			kafkaMsg = &sarama.ProducerMessage{
				Key:       sarama.StringEncoder(partitionKey),
				Partition: partitionNumber,
				Value:     sarama.ByteEncoder(serialized),
				Topic:     topic,
				Timestamp: reported,
				Metadata:  message.Uid,
			}

			if producer == nil {
				v, _ := kafkaMsg.Value.Encode()
				pkey, _ := kafkaMsg.Key.Encode()
				fwder.logger.Info("Message", "partitionkey", string(pkey), "topic", kafkaMsg.Topic, "msgid", message.Uid)
				fmt.Fprintln(os.Stderr, string(v))
				from.ACK(message.Uid)
			} else {
				producer.Input() <- kafkaMsg
			}
			ffjson.Pool(serialized)
		}
	}
}

func (fwder *kafkaForwarder) getProducer(ctx context.Context, to *conf.KafkaConfig) sarama.AsyncProducer {
	var producer sarama.AsyncProducer
	var err error
	for {
		producer, err = to.GetAsyncProducer()
		if err == nil {
			fwder.logger.Debug("Got a Kafka producer")
			return producer
		} else {
			if fwder.metrics != nil {
				fwder.metrics.KafkaConnectionErrorCounter.Inc()
			}
			fwder.logger.Warn("Error getting a Kafka client", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (fwder *kafkaForwarder) listenKafkaResponses(from Store, succChan <-chan *sarama.ProducerMessage, failChan <-chan *sarama.ProducerError) {
	defer fwder.wg.Done()

	once := &sync.Once{}

	for {
		if succChan == nil && failChan == nil {
			return
		}
		select {
		case succ, more := <-succChan:
			if more {
				from.ACK(succ.Metadata.(string))
				if fwder.metrics != nil {
					fwder.metrics.KafkaAckNackCounter.WithLabelValues("ack", succ.Topic).Inc()
				}
			} else {
				succChan = nil
			}

		case fail, more := <-failChan:
			if more {
				from.NACK(fail.Msg.Metadata.(string))
				fwder.logger.Info("Kafka producer error", "error", fail.Error())
				if model.IsFatalKafkaError(fail.Err) {
					once.Do(func() { close(fwder.errorChan) })
				}
				if fwder.metrics != nil {
					fwder.metrics.KafkaAckNackCounter.WithLabelValues("nack", fail.Msg.Topic).Inc()
				}
			} else {
				failChan = nil
			}
		}
	}

}
