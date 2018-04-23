package clients

import (
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils/queue"
)

type KafkaClient struct {
}

func (k *KafkaClient) Connect() error {
	panic("not implemented")
}

func (k *KafkaClient) Close() error {
	panic("not implemented")
}

func (k *KafkaClient) Send(msg *model.FullMessage) error {
	panic("not implemented")
}

func (k *KafkaClient) Flush() error {
	panic("not implemented")
}

func (k *KafkaClient) Ack() *queue.AckQueue {
	panic("not implemented")
}

func (k *KafkaClient) Nack() *queue.AckQueue {
	panic("not implemented")
}
