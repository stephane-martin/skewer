package dests

import (
	"context"
	"time"

	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/model/encoders"
)

type UDPDestination struct {
	*baseDestination
	client *clients.SyslogUDPClient
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

	d.client = client

	rebind := e.config.UDPDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(rebind):
				e.logger.Info("UDP destination rebind period has expired", "rebind", rebind.String())
				d.dofatal()
			}
		}()
	}

	return d, nil
}

func (d *UDPDestination) Send(message *model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	uid := message.Uid
	err = d.client.Send(message)

	// careful not to use message afterwards
	if err == nil {
		d.ack(uid)
		return nil
	} else if encoders.IsEncodingError(err) {
		d.permerr(uid)
		return err
	} else {
		// error writing to the UDP conn
		d.nack(uid)
		d.dofatal()
		return err
	}
}

func (d *UDPDestination) Close() error {
	return d.client.Close()
}
