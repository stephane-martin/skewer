package store

import (
	"context"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	sarama "gopkg.in/Shopify/sarama.v1"
)

type kafkaDestination struct {
	producer   sarama.AsyncProducer
	logger     log15.Logger
	fatal      chan struct{}
	registry   *prometheus.Registry
	ackCounter *prometheus.CounterVec
	once       sync.Once
	ack        storeCallback
	nack       storeCallback
	permerr    storeCallback
}

func NewKafkaDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) Destination {
	d := &kafkaDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
	}
	d.ackCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_dest_kafka_ack_total",
			Help: "number of kafka acknowledgments",
		},
		[]string{"status", "topic"},
	)
	d.registry.MustRegister(d.ackCounter)
	var err error
	for {
		d.producer, err = bc.Kafka.GetAsyncProducer()
		if err == nil {
			logger.Info("The forwarder got a Kafka producer")
			break
		}
		//fwder.metrics.KafkaConnectionErrorCounter.Inc()
		logger.Debug("Error getting a Kafka client", "error", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	d.fatal = make(chan struct{})

	go func() {
		var m *sarama.ProducerMessage
		for m = range d.producer.Successes() {
			d.ack(m.Metadata.(ulid.ULID))
			d.ackCounter.WithLabelValues("ack", m.Topic).Inc()
		}
	}()

	go func() {
		var m *sarama.ProducerError
		for m = range d.producer.Errors() {
			d.nack(m.Msg.Metadata.(ulid.ULID))
			d.ackCounter.WithLabelValues("nack", m.Msg.Topic).Inc()
			if model.IsFatalKafkaError(m.Err) {
				d.once.Do(func() { close(d.fatal) })
			}
		}
	}()

	return d
}

func (d *kafkaDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}

func (d *kafkaDestination) Send(message *model.TcpUdpParsedMessage, partitionKey string, partitionNumber int32, topic string) error {
	reported := time.Unix(0, message.Parsed.Fields.TimeReportedNum).UTC()
	message.Parsed.Fields.TimeGenerated = time.Unix(0, message.Parsed.Fields.TimeGeneratedNum).UTC().Format(time.RFC3339Nano)
	message.Parsed.Fields.TimeReported = reported.Format(time.RFC3339Nano)

	serialized, err := ffjson.Marshal(&message.Parsed)

	if err != nil {
		d.permerr(message.Uid)
		return err
	}

	kafkaMsg := &sarama.ProducerMessage{
		Key:       sarama.StringEncoder(partitionKey),
		Partition: partitionNumber,
		Value:     sarama.ByteEncoder(serialized),
		Topic:     topic,
		Timestamp: reported,
		Metadata:  message.Uid,
	}
	d.producer.Input() <- kafkaMsg
	//ffjson.Pool(serialized)
	return nil
}

func (d *kafkaDestination) Close() {
	d.producer.AsyncClose()
}

func (d *kafkaDestination) Fatal() chan struct{} {
	return d.fatal
}
