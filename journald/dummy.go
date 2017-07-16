// +build !linux nonsystemd

package journald

import (
	"context"

	"github.com/inconshreveable/log15"
)

var Supported bool = false

type JournaldReader interface {
	Start()
	Stop()
	Entries() chan map[string]string
}

type reader struct {
	entries chan map[string]string
}

func NewReader(ctx context.Context, logger log15.Logger) (JournaldReader, error) {
	r := &reader{}
	r.entries = make(chan map[string]string)
	return r, nil
}

func (r *reader) Start() {}
func (r *reader) Stop()  {}

func (r *reader) Entries() chan map[string]string {
	return r.entries
}
