package store

import (
	"context"
	"fmt"

	"github.com/inconshreveable/log15"
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
	GetSyslogConfig(configID ulid.ULID) (*conf.SyslogConfig, error)
	StoreAllSyslogConfigs(c conf.BaseConfig) error
	ReadAllBadgers() (map[string]string, map[string]string, map[string]string)
	Gather() ([]*dto.MetricFamily, error)
	ReleaseMsg(msg *model.FullMessage)
	Destinations() []conf.DestinationType
}

type Forwarder interface {
	Forward(ctx context.Context, from Store, bc conf.BaseConfig)
	Fatal() chan struct{}
	WaitFinished()
	Gather() ([]*dto.MetricFamily, error)
}

type Destination interface {
	Send(m model.FullMessage, partitionKey string, partitionNumber int32, topic string) error
	Fatal() chan struct{}
	Close()
	Gather() ([]*dto.MetricFamily, error)
}

func NewDestination(
	ctx context.Context,
	typ conf.DestinationType,
	bc conf.BaseConfig,
	ack, nack, permerr storeCallback,
	logger log15.Logger) (Destination, error) {

	switch typ {
	case conf.Kafka:
		return NewKafkaDestination(ctx, bc, ack, nack, permerr, logger)
	case conf.Udp:
		return NewUdpDestination(ctx, bc, ack, nack, permerr, logger)
	case conf.Tcp:
		return NewTcpDestination(ctx, bc, ack, nack, permerr, logger)
	case conf.Relp:
		return NewRelpDestination(ctx, bc, ack, nack, permerr, logger)
	case conf.File:
		return NewFileDestination(ctx, bc, ack, nack, permerr, logger)
	case conf.Stderr:
		return NewStderrDestination(ctx, bc, ack, nack, permerr, logger)
	default:
		return nil, fmt.Errorf("Unknown destination type: %d", typ)
	}
}
