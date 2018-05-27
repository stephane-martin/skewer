package dests

import (
	"context"
	"sync"

	sarama "github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/valyala/bytebufferpool"
)

type KafkaDestination struct {
	*baseDestination
	producer   sarama.AsyncProducer
	collectors []prometheus.Collector
	wg         sync.WaitGroup
}

func NewKafkaDestination(ctx context.Context, e *Env) (Destination, error) {
	d := &KafkaDestination{
		baseDestination: newBaseDestination(conf.Kafka, "kafka", e),
	}
	err := d.setFormat(e.config.KafkaDest.Format)
	if err != nil {
		return nil, err
	}

	producer, registry, err := e.config.KafkaDest.GetAsyncProducer(e.confined)
	if err != nil {
		connCounter.WithLabelValues("kafka", "fail").Inc()
		return nil, err
	}
	// we've got a kafka client
	d.producer = producer
	// record the success
	connCounter.WithLabelValues("kafka", "success").Inc()
	// register the kafka client metrics
	d.collectors = utils.KafkaProducerMetrics(registry, "skw_dest_kafka")
	Registry.MustRegister(d.collectors...)

	// process the kafka acks
	d.wg.Add(1)
	go func() {
		for m := range d.producer.Successes() {
			d.ACK(m.Metadata.(utils.MyULID))
		}
		d.wg.Done()
	}()

	// process the kafka errors
	d.wg.Add(1)
	go func() {
		for m := range d.producer.Errors() {
			d.NACK(m.Msg.Metadata.(utils.MyULID))
			if model.IsFatalKafkaError(m.Err) {
				d.dofatal(eerrors.Wrap(m.Err, "Kafka fatal error"))
			}
		}
		d.wg.Done()
	}()

	// unregister metrics when the client has finished all operations
	go func() {
		d.wg.Wait()
		for _, collector := range d.collectors {
			Registry.Unregister(collector)
		}
		d.collectors = nil
	}()

	return d, nil
}

func (d *KafkaDestination) sendOne(ctx context.Context, message *model.FullMessage, topic, pKey string, pNumber int32) (err error) {
	buf := bytebufferpool.Get()
	err = d.encoder(message, buf)
	if err != nil {
		bytebufferpool.Put(buf)
		return err
	}
	// we use buf.String() to get a copy of the buffer, so that we can push back the buffer to the pool
	kafkaMsg := &sarama.ProducerMessage{
		Key:       sarama.StringEncoder(pKey),
		Partition: pNumber,
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
	d.wg.Wait()
	return nil
}

func (d *KafkaDestination) Send(ctx context.Context, msgs []model.OutputMsg) (err eerrors.ErrorSlice) {
	return d.ForEachWithTopic(ctx, d.sendOne, false, true, msgs)
}
