package dests

import (
	"context"
	"time"

	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue"
)

type RELPDestination struct {
	*baseDestination
	clt *clients.RELPClient
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

	d.clt = clt

	rebind := e.config.RELPDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
				d.clt.Close()
			case <-time.After(rebind):
				d.dofatal(eerrors.Errorf("Rebind period has expired (%s)", rebind.String()))
			}
		}()
	}

	go func() {
		ackChan := d.clt.Ack()
		nackChan := d.clt.Nack()
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
				d.dofatal(eerrors.Errorf("RELP server returned a NACK for UID '%s'", uid.String()))
			}
		}
	}()

	return d, nil
}

func (d *RELPDestination) Close() (err error) {
	return d.clt.Close()
}

func (d *RELPDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err eerrors.ErrorSlice) {
	return d.ForEach(ctx, d.clt.Send, false, true, msgs)
}
