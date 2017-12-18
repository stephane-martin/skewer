package dests

import (
	"context"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

var sp = []byte(" ")
var zero ulid.ULID

type tcpDestination struct {
	logger      log15.Logger
	fatal       chan struct{}
	registry    *prometheus.Registry
	ack         storeCallback
	nack        storeCallback
	permerr     storeCallback
	previousUid ulid.ULID
	clt         *clients.SyslogTCPClient
	once        sync.Once
}

func NewTcpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	clt := clients.NewSyslogTCPClient().
		Host(bc.TcpDest.Host).
		Port(bc.TcpDest.Port).
		Path(bc.TcpDest.UnixSocketPath).
		Format(bc.TcpDest.Format).
		KeepAlive(bc.TcpDest.KeepAlive).
		KeepAlivePeriod(bc.TcpDest.KeepAlivePeriod).
		LineFraming(bc.TcpDest.LineFraming).
		FrameDelimiter(bc.TcpDest.FrameDelimiter).
		ConnTimeout(bc.TcpDest.ConnTimeout).
		FlushPeriod(bc.TcpDest.FlushPeriod)

	err = clt.Connect()
	if err != nil {
		return nil, err
	}

	d := &tcpDestination{
		logger:   logger,
		fatal:    make(chan struct{}),
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
		clt:      clt,
	}

	rebind := bc.TcpDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
			case <-time.After(rebind):
				logger.Info("TCP destination rebind period has expired", "rebind", rebind.String())
				d.once.Do(func() { close(d.fatal) })
			}
		}()
	}

	return d, nil
}

func (d *tcpDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.clt.Send(&message)
	if err == nil {
		if d.previousUid != zero {
			d.ack(d.previousUid, conf.Tcp)
		}
		d.previousUid = message.Uid
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid, conf.Tcp)
	} else {
		// error writing to the TCP conn
		d.nack(message.Uid, conf.Tcp)
		if d.previousUid != zero {
			d.nack(d.previousUid, conf.Tcp)
			d.previousUid = zero
		}
		d.once.Do(func() { close(d.fatal) })
	}
	return
}

func (d *tcpDestination) Close() error {
	return d.clt.Close()
}

func (d *tcpDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *tcpDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}
