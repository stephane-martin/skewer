// +build !linux nonsystemd

package journald

import (
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
)

var Supported = false

type DummyReader struct {
}

func NewReader(stasher base.Stasher, logger log15.Logger) (*DummyReader, error) {
	return new(DummyReader), nil
}

func (r *DummyReader) Start(utils.MyULID)        {}
func (r *DummyReader) Stop()                     {}
func (r *DummyReader) Shutdown()                 {}
func (r *DummyReader) FatalError() chan struct{} { return nil }
