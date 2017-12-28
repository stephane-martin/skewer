package services

import "github.com/stephane-martin/skewer/model"
import dto "github.com/prometheus/client_model/go"

type Provider interface {
	Start() ([]model.ListenerInfo, error)
	Stop()
	Shutdown()
	Gather() ([]*dto.MetricFamily, error)
	FatalError() chan struct{}
}
