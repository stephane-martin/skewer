package dests

import (
	"context"
	"time"

	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
)

type RELPDestination struct {
	*baseDestination
	client *clients.RELPClient
}

func NewRELPDestination(ctx context.Context, e *Env) (d *RELPDestination, err error) {
	d = &RELPDestination{
		baseDestination: newBaseDestination(conf.RELP, "relp", e),
	}
	err = d.setFormat(e.config.RELPDest.Format)
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
				d.ack(uid)
			}
			for {
				uid, _, err = nackChan.Get()
				if err != nil {
					break
				}
				d.nack(uid)
				d.logger.Info("RELP server returned a NACK", "uid", uid.String())
				d.dofatal()
			}
		}
	}()

	return d, nil
}

func (d *RELPDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.client.Send(&message)
	if err != nil {
		// the client send queue has been disposed
		d.nack(message.Uid)
		d.dofatal()
	}
	return
}

func (d *RELPDestination) Close() (err error) {
	return d.client.Close()
}
