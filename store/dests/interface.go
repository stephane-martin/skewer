package dests

import (
	"context"
	"fmt"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type Destination interface {
	Send(m model.FullMessage, partitionKey string, partitionNumber int32, topic string) error
	Fatal() chan struct{}
	Close() error
}

func NewDestination(
	ctx context.Context,
	typ conf.DestinationType,
	bc conf.BaseConfig,
	ack, nack, permerr storeCallback,
	confined bool,
	logger log15.Logger) (Destination, error) {

	switch typ {
	case conf.Kafka:
		return NewKafkaDestination(ctx, confined, bc, ack, nack, permerr, logger)
	case conf.UDP:
		return NewUdpDestination(ctx, confined, bc, ack, nack, permerr, logger)
	case conf.TCP:
		return NewTcpDestination(ctx, confined, bc, ack, nack, permerr, logger)
	case conf.RELP:
		return NewRelpDestination(ctx, confined, bc, ack, nack, permerr, logger)
	case conf.File:
		return NewFileDestination(ctx, confined, bc, ack, nack, permerr, logger)
	case conf.Stderr:
		return NewStderrDestination(ctx, confined, bc, ack, nack, permerr, logger)
	case conf.Graylog:
		return NewGraylogDestination(ctx, confined, bc, ack, nack, permerr, logger)
	case conf.HTTP:
		return NewHTTPDestination(ctx, confined, bc, ack, nack, permerr, logger)
	default:
		return nil, fmt.Errorf("Unknown destination type: %d", typ)
	}
}
