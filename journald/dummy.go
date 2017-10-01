// +build !linux nonsystemd

package journald

import (
	"github.com/inconshreveable/log15"
)

var Supported bool = false

type JournaldReader interface {
	Start(coding string)
	Stop()
	Shutdown()
	Entries() chan map[string]string
}

type reader struct {
	entries chan map[string]string
}

func NewReader(logger log15.Logger) (JournaldReader, error) {
	r := &reader{}
	r.entries = make(chan map[string]string)
	return r, nil
}

func (r *reader) Start(coding string) {}
func (r *reader) Stop()               {}
func (r *reader) Shutdown()           {}

func (r *reader) Entries() chan map[string]string {
	return r.entries
}
