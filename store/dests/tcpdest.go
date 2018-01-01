package dests

import (
	"context"
	"time"

	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

var sp = []byte(" ")

type TCPDestination struct {
	*baseDestination
	previousUid ulid.ULID
	clt         *clients.SyslogTCPClient
}

func NewTCPDestination(ctx context.Context, e *Env) (d *TCPDestination, err error) {
	clt := clients.NewSyslogTCPClient(e.logger).
		Host(e.config.TCPDest.Host).
		Port(e.config.TCPDest.Port).
		Path(e.config.TCPDest.UnixSocketPath).
		Format(e.config.TCPDest.Format).
		KeepAlive(e.config.TCPDest.KeepAlive).
		KeepAlivePeriod(e.config.TCPDest.KeepAlivePeriod).
		LineFraming(e.config.TCPDest.LineFraming).
		FrameDelimiter(e.config.TCPDest.FrameDelimiter).
		ConnTimeout(e.config.TCPDest.ConnTimeout).
		FlushPeriod(e.config.TCPDest.FlushPeriod)

	if e.config.TCPDest.TLSEnabled {
		config, err := utils.NewTLSConfig(
			e.config.TCPDest.Host,
			e.config.TCPDest.CAFile,
			e.config.TCPDest.CAPath,
			e.config.TCPDest.CertFile,
			e.config.TCPDest.KeyFile,
			e.config.TCPDest.Insecure,
			e.confined,
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
		baseDestination: newBaseDestination(conf.TCP, "tcp", e),
		clt:             clt,
	}

	rebind := e.config.TCPDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
			case <-time.After(rebind):
				e.logger.Info("TCP destination rebind period has expired", "rebind", rebind.String())
				d.dofatal()
			}
		}()
	}

	return d, nil
}

func (d *TCPDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.clt.Send(&message)
	if err == nil {
		if d.previousUid != utils.ZeroUid {
			d.ack(d.previousUid)
		}
		d.previousUid = message.Uid
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid)
	} else {
		// error writing to the TCP conn
		d.nack(message.Uid)
		if d.previousUid != utils.ZeroUid {
			d.nack(d.previousUid)
			d.previousUid = utils.ZeroUid
		}
		d.dofatal()
	}
	return
}

func (d *TCPDestination) Close() error {
	return d.clt.Close()
}
