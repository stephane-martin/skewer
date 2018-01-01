package base

import (
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type Provider interface {
	Start() ([]model.ListenerInfo, error)
	Stop()
	Shutdown()
	Gather() ([]*dto.MetricFamily, error)
	FatalError() chan struct{}
	Type() Types
	SetConf(c conf.BaseConfig)
}
