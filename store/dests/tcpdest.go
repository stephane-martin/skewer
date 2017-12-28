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

type tcpDestination struct {
	logger      log15.Logger
	fatal       chan struct{}
	ack         storeCallback
	nack        storeCallback
	permerr     storeCallback
	previousUid ulid.ULID
	clt         *clients.SyslogTCPClient
	once        sync.Once
}

func NewTcpDestination(ctx context.Context, cfnd bool, bc conf.BaseConfig, ack, nack, permerr storeCallback, l log15.Logger) (dest Destination, err error) {
	clt := clients.NewSyslogTCPClient(l).
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

	if bc.TcpDest.TLSEnabled {
		config, err := utils.NewTLSConfig(
			bc.TcpDest.Host,
			bc.TcpDest.CAFile,
			bc.TcpDest.CAPath,
			bc.TcpDest.CertFile,
			bc.TcpDest.KeyFile,
			bc.TcpDest.Insecure,
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

	d := &tcpDestination{
		logger:  l,
		fatal:   make(chan struct{}),
		ack:     ack,
		nack:    nack,
		permerr: permerr,
		clt:     clt,
	}

	rebind := bc.TcpDest.Rebind
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

func (d *tcpDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.clt.Send(&message)
	if err == nil {
		if d.previousUid != utils.ZeroUid {
			ackCounter.WithLabelValues("tcp", "ack", "").Inc()
			d.ack(d.previousUid, conf.Tcp)
		}
		d.previousUid = message.Uid
	} else if model.IsEncodingError(err) {
		ackCounter.WithLabelValues("tcp", "permerr", "").Inc()
		d.permerr(message.Uid, conf.Tcp)
	} else {
		// error writing to the TCP conn
		ackCounter.WithLabelValues("tcp", "nack", "").Inc()
		d.nack(message.Uid, conf.Tcp)
		if d.previousUid != utils.ZeroUid {
			ackCounter.WithLabelValues("tcp", "nack", "").Inc()
			d.nack(d.previousUid, conf.Tcp)
			d.previousUid = utils.ZeroUid
		}
		fatalCounter.WithLabelValues("tcp").Inc()
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
