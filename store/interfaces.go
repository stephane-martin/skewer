package store

import (
	"context"

	"github.com/oklog/ulid"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type Store interface {
	Stash(m model.FullMessage) (error, error)
	Outputs(dest conf.DestinationType) chan *model.FullMessage
	ACK(uid ulid.ULID, dest conf.DestinationType)
	NACK(uid ulid.ULID, dest conf.DestinationType)
	PermError(uid ulid.ULID, dest conf.DestinationType)
	Errors() chan struct{}
	WaitFinished()
	GetSyslogConfig(configID ulid.ULID) (*conf.FilterSubConfig, error)
	StoreAllSyslogConfigs(c conf.BaseConfig) error
	ReadAllBadgers() (map[string]string, map[string]string, map[string]string)
	ReleaseMsg(msg *model.FullMessage)
	Destinations() []conf.DestinationType
	Confined() bool
}

type Forwarder interface {
	Forward(ctx context.Context)
	Fatal() chan struct{}
	WaitFinished()
	Gather() ([]*dto.MetricFamily, error)
}
