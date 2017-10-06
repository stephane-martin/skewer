// +build !linux nonsystemd

package journald

import (
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/model"
)

var Supported bool = false

type JournaldReader interface {
	Start(coding string)
	Stop()
	Shutdown()
	Entries() chan model.TcpUdpParsedMessage
}

type reader struct {
	entries chan model.TcpUdpParsedMessage
}

func NewReader(generator chan ulid.ULID, logger log15.Logger) (JournaldReader, error) {
	r := &reader{}
	r.entries = make(chan model.TcpUdpParsedMessage)
	return r, nil
}

func (r *reader) Start(coding string) {}
func (r *reader) Stop()               {}
func (r *reader) Shutdown()           {}

func (r *reader) Entries() chan model.TcpUdpParsedMessage {
	return r.entries
}
