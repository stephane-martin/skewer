package dests

import (
	"bytes"
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

type KafkaDestination struct {
	producer sarama.AsyncProducer
	logger   log15.Logger
	fatal    chan struct{}
	once     sync.Once
	ack      storeCallback
	nack     storeCallback
	permerr  storeCallback
	encoder  model.Encoder
}

func NewKafkaDestination(ctx context.Context, cfnd bool, bc conf.BaseConfig, ack, nack, pe storeCallback, l log15.Logger) (d *KafkaDestination, err error) {
	d = &KafkaDestination{
		logger:  l,
		ack:     ack,
		nack:    nack,
		permerr: pe,
		fatal:   make(chan struct{}),
	}
	d.encoder, err = model.NewEncoder(bc.KafkaDest.Format)
	if err != nil {
		return nil, err
	}
	for {
		d.producer, err = bc.KafkaDest.GetAsyncProducer(cfnd)
		if err == nil {
			connCounter.WithLabelValues("kafka", "success").Inc()
			l.Info("The forwarder got a Kafka producer")
			break
		}
		connCounter.WithLabelValues("kafka", "fail").Inc()
		l.Debug("Error getting a Kafka client", "error", err)
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

func (d *KafkaDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	buf := bytes.NewBuffer(nil)
	err = d.encoder.Enc(message, buf)
	if err != nil {
		ackCounter.WithLabelValues("kafka", "permerr", topic).Inc()
		d.permerr(message.Uid, conf.Kafka)
		return err
	}
	kafkaMsg := &sarama.ProducerMessage{
		Key:       sarama.StringEncoder(partitionKey),
		Partition: partitionNumber,
		Value:     sarama.ByteEncoder(buf.Bytes()),
		Topic:     topic,
		Timestamp: message.Parsed.Fields.GetTimeReported(),
		Metadata:  message.Uid,
	}
	d.producer.Input() <- kafkaMsg
	kafkaInputsCounter.Inc()
	return nil
}

func (d *KafkaDestination) Close() error {
	d.producer.AsyncClose()
	return nil
}

func (d *KafkaDestination) Fatal() chan struct{} {
	return d.fatal
}
