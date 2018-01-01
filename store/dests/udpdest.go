package dests

import (
	"context"
	"time"

	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type UDPDestination struct {
	*baseDestination
	client *clients.SyslogUDPClient
}

func NewUDPDestination(ctx context.Context, e *Env) (d *UDPDestination, err error) {
	client := clients.NewSyslogUDPClient(e.logger).
		Host(e.config.UDPDest.Host).
		Port(e.config.UDPDest.Port).
		Path(e.config.UDPDest.UnixSocketPath).
		Format(e.config.UDPDest.Format)

	err = client.Connect()
	if err != nil {
		connCounter.WithLabelValues("udp", "fail").Inc()
		return nil, err
	}
	connCounter.WithLabelValues("udp", "success").Inc()

	d = &UDPDestination{
		baseDestination: newBaseDestination(conf.UDP, "udp", e),
		client:          client,
	}

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

func (d *UDPDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.client.Send(&message)
	if err == nil {
		d.ack(message.Uid)
		return nil
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid)
		return err
	} else {
		// error writing to the UDP conn
		d.nack(message.Uid)
		d.dofatal()
		return err
	}
}

func (d *UDPDestination) Close() error {
	return d.client.Close()
}
