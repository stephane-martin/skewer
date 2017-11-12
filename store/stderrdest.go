package store

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type stderrDestination struct {
	logger   log15.Logger
	fatal    chan struct{}
	registry *prometheus.Registry
	once     sync.Once
	ack      storeCallback
	nack     storeCallback
	permerr  storeCallback
	format   string
	encoder  model.Encoder
}

func NewStderrDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	d := &stderrDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
		format:   bc.FileDest.Format,
		fatal:    make(chan struct{}),
	}

	d.encoder, err = model.NewEncoder(os.Stderr, d.format)
	if err != nil {
		return nil, fmt.Errorf("Error getting encoder: %s", err)
	}

	return d, nil
}

func (d *stderrDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = model.ChainEncode(d.encoder, &message, "\n")
	if err == nil {
		d.ack(message.Uid)
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid)
	} else {
		d.nack(message.Uid)
	}
	return err
}

func (d *stderrDestination) Close() {}

func (d *stderrDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *stderrDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}
