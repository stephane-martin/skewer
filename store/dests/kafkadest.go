package dests

import (
	"bytes"
	"context"
	"fmt"
	"time"

	sarama "github.com/Shopify/sarama"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

type KafkaDestination struct {
	*baseDestination
	producer sarama.AsyncProducer
}

func NewKafkaDestination(ctx context.Context, e *Env) (d *KafkaDestination, err error) {
	d = &KafkaDestination{
		baseDestination: newBaseDestination(conf.Kafka, "kafka", e),
	}
	err = d.setFormat(e.config.KafkaDest.Format)
	if err != nil {
		return nil, err
	}
	for {
		d.producer, err = e.config.KafkaDest.GetAsyncProducer(e.confined)
		if err == nil {
			connCounter.WithLabelValues("kafka", "success").Inc()
			e.logger.Info("The forwarder got a Kafka producer")
			break
		}
		connCounter.WithLabelValues("kafka", "fail").Inc()
		e.logger.Debug("Error getting a Kafka client", "error", err)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("Kafka destination aborted: %s", err.Error())
		case <-time.After(5 * time.Second):
		}
	}

	go func() {
		var m *sarama.ProducerMessage
		for m = range d.producer.Successes() {
			d.ack(m.Metadata.(utils.MyULID))
		}
	}()

	go func() {
		var m *sarama.ProducerError
		for m = range d.producer.Errors() {
			d.nack(m.Msg.Metadata.(utils.MyULID))
			if model.IsFatalKafkaError(m.Err) {
				d.dofatal()
			}
		}
	}()

	return d, nil
}

func (d *KafkaDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	buf := bytes.NewBuffer(nil)
	err = d.encoder(message, buf)
	if err != nil {
		d.permerr(message.Uid)
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
