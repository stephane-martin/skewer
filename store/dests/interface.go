package dests

import (
	"context"
	"fmt"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

type Destination interface {
	Send(ctx context.Context, m []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) error
	Fatal() chan struct{}
	Close() error
	ACK(utils.MyULID)
	NACK(utils.MyULID)
	PermError(utils.MyULID)
	NACKAllSlice([]*model.FullMessage)
}

type constructor func(ctx context.Context, e *Env) (Destination, error)

var destinations = map[conf.DestinationType]constructor{
	conf.Kafka:           NewKafkaDestination,
	conf.UDP:             NewUDPDestination,
	conf.TCP:             NewTCPDestination,
	conf.RELP:            NewRELPDestination,
	conf.File:            NewFileDestination,
	conf.Stderr:          NewStderrDestination,
	conf.Graylog:         NewGraylogDestination,
	conf.HTTP:            NewHTTPDestination,
	conf.NATS:            NewNATSDestination,
	conf.HTTPServer:      NewHTTPServerDestination,
	conf.WebsocketServer: NewWebsocketServerDestination,
	conf.Elasticsearch:   NewElasticDestination,
	conf.Redis:           NewRedisDestination,
}

func NewDestination(ctx context.Context, typ conf.DestinationType, e *Env) (Destination, error) {
	if c, ok := destinations[typ]; ok {
		return c(ctx, e)
	}
	return nil, fmt.Errorf("Unknown destination type: %d", typ)
}
