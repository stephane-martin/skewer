package dests

import (
	"context"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type relpDestination struct {
	logger     log15.Logger
	fatal      chan struct{}
	registry   *prometheus.Registry
	client     *clients.RELPClient
	once       sync.Once
	ack        storeCallback
	nack       storeCallback
	permerr    storeCallback
	ackCounter *prometheus.CounterVec
}

func NewRelpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	clt := clients.NewRELPClient(logger).
		Host(bc.RelpDest.Host).
		Port(bc.RelpDest.Port).
		Path(bc.RelpDest.UnixSocketPath).
		Format(bc.RelpDest.Format).
		KeepAlive(bc.RelpDest.KeepAlive).
		KeepAlivePeriod(bc.RelpDest.KeepAlivePeriod).
		ConnTimeout(bc.RelpDest.ConnTimeout).
		RelpTimeout(bc.RelpDest.RelpTimeout).
		WindowSize(bc.RelpDest.WindowSize).
		FlushPeriod(bc.RelpDest.FlushPeriod)

	err = clt.Connect()
	if err != nil {
		return nil, err
	}

	d := &relpDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
		fatal:    make(chan struct{}),
		client:   clt,
	}

	d.ackCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relpdest_ack_total",
			Help: "number of RELP acknowledgments",
		},
		[]string{"status"},
	)
	d.registry.MustRegister(d.ackCounter)

	rebind := bc.RelpDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
			case <-time.After(rebind):
				logger.Info("RELP destination rebind period has expired", "rebind", rebind.String())
				d.once.Do(func() { close(d.fatal) })
			}
		}()
	}

	go func() {
		// TODO: metrics
		ackChan := d.client.Ack()
		nackChan := d.client.Nack()
		for {
			if ackChan == nil && nackChan == nil {
				return
			}
			select {
			case uid, more := <-ackChan:
				if more {
					d.ack(uid, conf.Relp)
				} else {
					ackChan = nil
				}
			case uid, more := <-nackChan:
				if more {
					d.nack(uid, conf.Relp)
					d.logger.Info("RELP server returned a NACK", "uid", uid.String())
					d.once.Do(func() { close(d.fatal) })
				} else {
					nackChan = nil
				}
			}
		}
	}()

	return d, nil
}

func (d *relpDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.client.Send(&message)
	if err != nil {
		// the client send queue has been disposed
		d.nack(message.Uid, conf.Relp)
		d.once.Do(func() { close(d.fatal) })
	}
	return
}

func (d *relpDestination) Close() (err error) {
	return d.client.Close()
}

func (d *relpDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *relpDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}
