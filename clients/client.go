package clients

import (
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/model"
)

type Client interface {
	Connect() error
	Close() error
	Send(msg *model.FullMessage) error
	Flush() error
	Ack() chan ulid.ULID
	Nack() chan ulid.ULID
}
