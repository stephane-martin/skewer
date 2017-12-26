package dests

import (
	"context"
	"fmt"
	"sync"
	"time"

	sarama "github.com/Shopify/sarama"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type kafkaDestination struct {
	producer sarama.AsyncProducer
	logger   log15.Logger
	fatal    chan struct{}
	once     sync.Once
	ack      storeCallback
	nack     storeCallback
	permerr  storeCallback
	format   string
}

func NewKafkaDestination(ctx context.Context, confined bool, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (Destination, error) {
	d := &kafkaDestination{
		logger:  logger,
		ack:     ack,
		nack:    nack,
		permerr: permerr,
		format:  bc.KafkaDest.Format,
		fatal:   make(chan struct{}),
	}
	var err error
	for {
		d.producer, err = bc.KafkaDest.GetAsyncProducer(confined)
		if err == nil {
			connCounter.WithLabelValues("kafka", "success").Inc()
			logger.Info("The forwarder got a Kafka producer")
			break
		}
		connCounter.WithLabelValues("kafka", "fail").Inc()
		logger.Debug("Error getting a Kafka client", "error", err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("Kafka destination aborted: %s", err.Error())
		case <-time.After(5 * time.Second):
		}
	}

	go func() {
		var m *sarama.ProducerMessage
		for m = range d.producer.Successes() {
			d.ack(m.Metadata.(ulid.ULID), conf.Kafka)
			ackCounter.WithLabelValues("kafka", "ack", m.Topic).Inc()
		}
	}()

	go func() {
		var m *sarama.ProducerError
		for m = range d.producer.Errors() {
			d.nack(m.Msg.Metadata.(ulid.ULID), conf.Kafka)
			ackCounter.WithLabelValues("kafka", "nack", m.Msg.Topic).Inc()
			if model.IsFatalKafkaError(m.Err) {
				fatalCounter.WithLabelValues("kafka").Inc()
				d.once.Do(func() { close(d.fatal) })
			}
		}
	}()

	return d, nil
}

func (d *kafkaDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) error {
	serialized, err := message.MarshalAll(d.format)

	if err != nil {
		ackCounter.WithLabelValues("kafka", "permerr", topic).Inc()
		d.permerr(message.Uid, conf.Kafka)
		return err
	}

	kafkaMsg := &sarama.ProducerMessage{
		Key:       sarama.StringEncoder(partitionKey),
		Partition: partitionNumber,
		Value:     sarama.ByteEncoder(serialized),
		Topic:     topic,
		Timestamp: message.Parsed.Fields.GetTimeReported(),
		Metadata:  message.Uid,
	}
	d.producer.Input() <- kafkaMsg
	kafkaInputsCounter.Inc()
	return nil
}

func (d *kafkaDestination) Close() error {
	d.producer.AsyncClose()
	return nil
}

func (d *kafkaDestination) Fatal() chan struct{} {
	return d.fatal
}
