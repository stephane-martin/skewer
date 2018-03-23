package dests

import (
	"context"
	"time"

	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
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
			case <-time.After(rebind):
				e.logger.Info("UDP destination rebind period has expired", "rebind", rebind.String())
				d.dofatal()
			}
		}()
	}

	return d, nil
}

func (d *UDPDestination) sendOne(ctx context.Context, message *model.FullMessage) (err error) {
	err = d.clt.Send(message)

	if err == nil {
		d.ACK(message.Uid)
		return nil
	}
	if encoders.IsEncodingError(err) {
		d.PermError(message.Uid)
		return err
	}
	// error writing to the UDP conn
	d.NACK(message.Uid)
	return err
}

func (d *UDPDestination) Close() error {
	return d.clt.Close()
}

func (d *UDPDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var e error
	var msg *model.FullMessage
	for {
		if len(msgs) == 0 {
			break
		}
		msg = msgs[0].Message
		msgs = msgs[1:]
		e = d.sendOne(ctx, msg)
		model.FullFree(msg)
		if e != nil && !encoders.IsEncodingError(e) {
			// network error, we stop now and NACK remaining messages
			for i := 0; i < len(msgs); i++ {
				d.NACK(msgs[i].Message.Uid)
				model.FullFree(msgs[i].Message)
			}
			d.dofatal()
			return e
		}
		if e != nil && err == nil {
			err = e
		}
	}
	return err
}
