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
