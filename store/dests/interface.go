package dests

import (
	"context"
	"fmt"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type Destination interface {
	Send(m model.FullMessage, partitionKey string, partitionNumber int32, topic string) error
	Fatal() chan struct{}
	Close() error
}

func NewDestination(ctx context.Context, typ conf.DestinationType, e *Env) (Destination, error) {
	switch typ {
	case conf.Kafka:
		return NewKafkaDestination(ctx, e)
	case conf.UDP:
		return NewUDPDestination(ctx, e)
	case conf.TCP:
		return NewTCPDestination(ctx, e)
	case conf.RELP:
		return NewRELPDestination(ctx, e)
	case conf.File:
		return NewFileDestination(ctx, e)
	case conf.Stderr:
		return NewStderrDestination(ctx, e)
	case conf.Graylog:
		return NewGraylogDestination(ctx, e)
	case conf.HTTP:
		return NewHTTPDestination(ctx, e)
	default:
		return nil, fmt.Errorf("Unknown destination type: %d", typ)
	}
}
