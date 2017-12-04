// +build !linux nonsystemd

package journald

import (
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/utils/queue"
)

var Supported bool = false

type reader struct {
	entries *queue.MessageQueue
}

func NewReader(generator chan ulid.ULID, logger log15.Logger) (JournaldReader, error) {
	r := &reader{}
	r.entries = queue.NewMessageQueue()
	return r, nil
}

func (r *reader) Start()    {}
func (r *reader) Stop()     {}
func (r *reader) Shutdown() {}

func (r *reader) Entries() *queue.MessageQueue {
	return r.entries
}
