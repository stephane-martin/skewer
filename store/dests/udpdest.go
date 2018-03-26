package dests

import (
	"context"
	"time"

	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

type UDPDestination struct {
	*baseDestination
	clt *clients.SyslogUDPClient
}

func NewUDPDestination(ctx context.Context, e *Env) (Destination, error) {
	d := &UDPDestination{
		baseDestination: newBaseDestination(conf.UDP, "udp", e),
	}
	err := d.setFormat(e.config.UDPDest.Format)
	if err != nil {
		return nil, err
	}
	client := clients.NewSyslogUDPClient(e.logger).
		Host(e.config.UDPDest.Host).
		Port(e.config.UDPDest.Port).
		Path(e.config.UDPDest.UnixSocketPath).
		Format(d.format)

	err = client.Connect()
	if err != nil {
		connCounter.WithLabelValues("udp", "fail").Inc()
		return nil, err
	}
	connCounter.WithLabelValues("udp", "success").Inc()

	d.clt = client

	rebind := e.config.UDPDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				d.clt.Close()
			case <-time.After(rebind):
				e.logger.Info("UDP destination rebind period has expired", "rebind", rebind.String())
				d.dofatal()
			}
		}()
	}

	return d, nil
}

func (d *UDPDestination) Close() error {
	return d.clt.Close()
}

func (d *UDPDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var e error
	var msg *model.FullMessage
	var uid utils.MyULID
	for len(msgs) > 0 {
		msg = msgs[0].Message
		uid = msg.Uid
		msgs = msgs[1:]
		e = d.clt.Send(msg)
		model.FullFree(msg)
		if e != nil {
			if encoders.IsEncodingError(e) {
				d.PermError(uid)
			} else {
				d.NACK(uid)
				d.NACKRemaining(msgs)
				d.dofatal()
				return e
			}
			if err == nil {
				err = e
			}
		}
	}
	return err
}
