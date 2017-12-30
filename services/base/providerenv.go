package base

import (
	"os"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/kring"
)

type ProviderEnv struct {
	Confined bool
	Profile  bool
	Ring     kring.Ring
	Reporter Reporter
	Binder   binder.Client
	Logger   log15.Logger
	Pipe     *os.File
}
