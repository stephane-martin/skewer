package services

import "github.com/stephane-martin/skewer/model"
import dto "github.com/prometheus/client_model/go"

type NetworkService interface {
	Start(test bool) ([]model.ListenerInfo, error)
	Stop()
	Shutdown()
	Gather() ([]*dto.MetricFamily, error)
}

type StoreService interface {
	NetworkService
	model.Stasher
	Errors() chan struct{}
}
