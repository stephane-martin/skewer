package store

import (
	"context"

	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type Store interface {
	Stash(m model.TcpUdpParsedMessage) (error, error)
	Outputs() chan *model.TcpUdpParsedMessage
	ACK(uid string)
	NACK(uid string)
	PermError(uid string)
	Errors() chan struct{}
	WaitFinished()
	GetSyslogConfig(configID string) (*conf.SyslogConfig, error)
	StoreAllSyslogConfigs(c conf.BaseConfig) error
	ReadAllBadgers() (map[string]string, map[string]string, map[string]string)
	Gather() ([]*dto.MetricFamily, error)
}

type Forwarder interface {
	Forward(ctx context.Context, from Store, to conf.KafkaConfig)
	ErrorChan() chan struct{}
	WaitFinished()
	Gather() ([]*dto.MetricFamily, error)
}
