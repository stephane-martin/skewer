package dests

import (
	"context"
	"fmt"
	"time"

	sarama "github.com/Shopify/sarama"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/valyala/bytebufferpool"
)

type KafkaDestination struct {
	*baseDestination
	producer sarama.AsyncProducer
}

func NewKafkaDestination(ctx context.Context, e *Env) (Destination, error) {
	d := &KafkaDestination{
		baseDestination: newBaseDestination(conf.Kafka, "kafka", e),
	}
	err := d.setFormat(e.config.KafkaDest.Format)
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

func (d *KafkaDestination) sendOne(message *model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	buf := bytebufferpool.Get()
	defer func() {
		bytebufferpool.Put(buf)
		model.FullFree(message)
	}()
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
		Timestamp: message.Fields.GetTimeReported(),
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

func (d *KafkaDestination) Send(msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var i int
	var e error
	for i = range msgs {
		e = d.sendOne(msgs[i].Message, msgs[i].PartitionKey, msgs[i].PartitionNumber, msgs[i].Topic)
		if e != nil {
			err = e
		}
	}
	return err
}
