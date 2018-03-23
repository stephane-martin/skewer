package dests

import (
	"context"
	"time"

	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
)

type RELPDestination struct {
	*baseDestination
	client *clients.RELPClient
}

func NewRELPDestination(ctx context.Context, e *Env) (Destination, error) {
	d := &RELPDestination{
		baseDestination: newBaseDestination(conf.RELP, "relp", e),
	}
	err := d.setFormat(e.config.RELPDest.Format)
	if err != nil {
		return nil, err
	}
	clt := clients.NewRELPClient(e.logger).
		Host(e.config.RELPDest.Host).
		Port(e.config.RELPDest.Port).
		Path(e.config.RELPDest.UnixSocketPath).
		Format(d.format).
		KeepAlive(e.config.RELPDest.KeepAlive).
		KeepAlivePeriod(e.config.RELPDest.KeepAlivePeriod).
		ConnTimeout(e.config.RELPDest.ConnTimeout).
		RelpTimeout(e.config.RELPDest.RelpTimeout).
		WindowSize(e.config.RELPDest.WindowSize).
		FlushPeriod(e.config.RELPDest.FlushPeriod)

	if e.config.RELPDest.TLSEnabled {
		config, err := utils.NewTLSConfig(
			e.config.RELPDest.Host,
			e.config.RELPDest.CAFile,
			e.config.RELPDest.CAPath,
			e.config.RELPDest.CertFile,
			e.config.RELPDest.KeyFile,
			e.config.RELPDest.Insecure,
			e.confined,
		)
		if err != nil {
			return nil, err
		}
		clt = clt.TLS(config)
	}

	err = clt.Connect()
	if err != nil {
		connCounter.WithLabelValues("relp", "fail").Inc()
		return nil, err
	}
	connCounter.WithLabelValues("relp", "success").Inc()

	d.client = clt

	rebind := e.config.RELPDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
			case <-time.After(rebind):
				e.logger.Info("RELP destination rebind period has expired", "rebind", rebind.String())
				d.dofatal()
			}
		}()
	}

	go func() {
		ackChan := d.client.Ack()
		nackChan := d.client.Nack()
		var err error
		var uid utils.MyULID

		for queue.WaitManyAckQueues(ackChan, nackChan) {
			for {
				uid, _, err = ackChan.Get()
				if err != nil {
					break
				}
				d.ACK(uid)
			}
			for {
				uid, _, err = nackChan.Get()
				if err != nil {
					break
				}
				d.NACK(uid)
				d.logger.Info("RELP server returned a NACK", "uid", uid.String())
				d.dofatal()
			}
		}
	}()

	return d, nil
}

func (d *RELPDestination) sendOne(ctx context.Context, message *model.FullMessage) (err error) {
	err = d.client.Send(ctx, message)

	if err != nil {
		if encoders.IsEncodingError(err) {
			d.PermError(message.Uid)
			return err
		}
		// error writing to the conn
		d.NACK(message.Uid)
		return err
	}
	// we do not ACK the message yet, as we have to wait for the server response
	return nil
}

func (d *RELPDestination) Close() (err error) {
	return d.client.Close()
}

func (d *RELPDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var msg *model.FullMessage
	var e error
	for {
		if len(msgs) == 0 {
			return
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
	return nil
}
