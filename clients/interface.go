package clients

import (
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils/queue"
)

type Client interface {
	Connect() error
	Close() error
	Send(msg *model.FullMessage) error
	Flush() error
	Ack() *queue.AckQueue
	Nack() *queue.AckQueue
}
