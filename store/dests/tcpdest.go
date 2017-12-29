package dests

import (
	"context"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

var sp = []byte(" ")

type TCPDestination struct {
	logger      log15.Logger
	fatal       chan struct{}
	ack         storeCallback
	nack        storeCallback
	permerr     storeCallback
	previousUid ulid.ULID
	clt         *clients.SyslogTCPClient
	once        sync.Once
}

func NewTCPDestination(ctx context.Context, cfnd bool, bc conf.BaseConfig, ack, nack, pe storeCallback, l log15.Logger) (d *TCPDestination, err error) {
	clt := clients.NewSyslogTCPClient(l).
		Host(bc.TCPDest.Host).
		Port(bc.TCPDest.Port).
		Path(bc.TCPDest.UnixSocketPath).
		Format(bc.TCPDest.Format).
		KeepAlive(bc.TCPDest.KeepAlive).
		KeepAlivePeriod(bc.TCPDest.KeepAlivePeriod).
		LineFraming(bc.TCPDest.LineFraming).
		FrameDelimiter(bc.TCPDest.FrameDelimiter).
		ConnTimeout(bc.TCPDest.ConnTimeout).
		FlushPeriod(bc.TCPDest.FlushPeriod)

	if bc.TCPDest.TLSEnabled {
		config, err := utils.NewTLSConfig(
			bc.TCPDest.Host,
			bc.TCPDest.CAFile,
			bc.TCPDest.CAPath,
			bc.TCPDest.CertFile,
			bc.TCPDest.KeyFile,
			bc.TCPDest.Insecure,
			cfnd,
		)
		if err != nil {
			return nil, err
		}
		clt = clt.TLS(config)
	}

	err = clt.Connect()
	if err != nil {
		connCounter.WithLabelValues("tcp", "fail").Inc()
		return nil, err
	}
	connCounter.WithLabelValues("tcp", "success").Inc()

	d = &TCPDestination{
		logger:  l,
		fatal:   make(chan struct{}),
		ack:     ack,
		nack:    nack,
		permerr: pe,
		clt:     clt,
	}

	rebind := bc.TCPDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
			case <-time.After(rebind):
				l.Info("TCP destination rebind period has expired", "rebind", rebind.String())
				d.once.Do(func() { close(d.fatal) })
			}
		}()
	}

	return d, nil
}

func (d *TCPDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.clt.Send(&message)
	if err == nil {
		if d.previousUid != utils.ZeroUid {
			ackCounter.WithLabelValues("tcp", "ack", "").Inc()
			d.ack(d.previousUid, conf.TCP)
		}
		d.previousUid = message.Uid
	} else if model.IsEncodingError(err) {
		ackCounter.WithLabelValues("tcp", "permerr", "").Inc()
		d.permerr(message.Uid, conf.TCP)
	} else {
		// error writing to the TCP conn
		ackCounter.WithLabelValues("tcp", "nack", "").Inc()
		d.nack(message.Uid, conf.TCP)
		if d.previousUid != utils.ZeroUid {
			ackCounter.WithLabelValues("tcp", "nack", "").Inc()
			d.nack(d.previousUid, conf.TCP)
			d.previousUid = utils.ZeroUid
		}
		fatalCounter.WithLabelValues("tcp").Inc()
		d.once.Do(func() { close(d.fatal) })
	}
	return
}

func (d *TCPDestination) Close() error {
	return d.clt.Close()
}

func (d *TCPDestination) Fatal() chan struct{} {
	return d.fatal
}
