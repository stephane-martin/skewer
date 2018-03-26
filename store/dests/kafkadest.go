package dests

import (
	"context"

	sarama "github.com/Shopify/sarama"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
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

	d.producer, err = e.config.KafkaDest.GetAsyncProducer(e.confined)
	if err != nil {
		connCounter.WithLabelValues("kafka", "fail").Inc()
		return nil, err
	}
	connCounter.WithLabelValues("kafka", "success").Inc()

	go func() {
		var m *sarama.ProducerMessage
		for m = range d.producer.Successes() {
			d.ACK(m.Metadata.(utils.MyULID))
		}
	}()

	go func() {
		var m *sarama.ProducerError
		for m = range d.producer.Errors() {
			d.NACK(m.Msg.Metadata.(utils.MyULID))
			if model.IsFatalKafkaError(m.Err) {
				d.dofatal()
			}
		}
	}()

	return d, nil
}

func (d *KafkaDestination) sendOne(message *model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	buf := bytebufferpool.Get()
	err = d.encoder(message, buf)
	if err != nil {
		bytebufferpool.Put(buf)
		return err
	}
	// we use buf.String() to get a copy of the buffer, so that we can push back the buffer to the pool
	kafkaMsg := &sarama.ProducerMessage{
		Key:       sarama.StringEncoder(partitionKey),
		Partition: partitionNumber,
		Value:     sarama.StringEncoder(buf.String()),
		Topic:     topic,
		Timestamp: message.Fields.GetTimeReported(),
		Metadata:  message.Uid,
	}
	bytebufferpool.Put(buf)
	d.producer.Input() <- kafkaMsg
	kafkaInputsCounter.Inc()
	return nil
}

func (d *KafkaDestination) Close() error {
	d.producer.AsyncClose()
	return nil
}

func (d *KafkaDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var e error
	var msg *model.FullMessage
	var uid utils.MyULID
	// we always send all messages to sarama
	for len(msgs) > 0 {
		msg = msgs[0].Message
		uid = msg.Uid
		e = d.sendOne(msg, msgs[0].PartitionKey, msgs[0].PartitionNumber, msgs[0].Topic)
		model.FullFree(msg)
		msgs = msgs[1:]
		if e != nil {
			if encoders.IsEncodingError(e) {
				d.PermError(uid)
				if err == nil {
					err = e
				}
			} else {
				d.NACK(uid)
				d.NACKRemaining(msgs)
				d.dofatal()
				return e
			}
		}
	}
	return err
}
