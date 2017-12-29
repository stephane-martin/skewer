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
	"github.com/stephane-martin/skewer/utils/queue"
)

type relpDestination struct {
	logger  log15.Logger
	fatal   chan struct{}
	client  *clients.RELPClient
	once    sync.Once
	ack     storeCallback
	nack    storeCallback
	permerr storeCallback
}

func NewRelpDestination(ctx context.Context, confined bool, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	clt := clients.NewRELPClient(logger).
		Host(bc.RELPDest.Host).
		Port(bc.RELPDest.Port).
		Path(bc.RELPDest.UnixSocketPath).
		Format(bc.RELPDest.Format).
		KeepAlive(bc.RELPDest.KeepAlive).
		KeepAlivePeriod(bc.RELPDest.KeepAlivePeriod).
		ConnTimeout(bc.RELPDest.ConnTimeout).
		RelpTimeout(bc.RELPDest.RelpTimeout).
		WindowSize(bc.RELPDest.WindowSize).
		FlushPeriod(bc.RELPDest.FlushPeriod)

	if bc.RELPDest.TLSEnabled {
		config, err := utils.NewTLSConfig(
			bc.RELPDest.Host,
			bc.RELPDest.CAFile,
			bc.RELPDest.CAPath,
			bc.RELPDest.CertFile,
			bc.RELPDest.KeyFile,
			bc.RELPDest.Insecure,
			confined,
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

	d := &relpDestination{
		logger:  logger,
		ack:     ack,
		nack:    nack,
		permerr: permerr,
		fatal:   make(chan struct{}),
		client:  clt,
	}

	rebind := bc.RELPDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
				// the store service asked for stop
			case <-time.After(rebind):
				logger.Info("RELP destination rebind period has expired", "rebind", rebind.String())
				d.once.Do(func() { close(d.fatal) })
			}
		}()
	}

	go func() {
		ackChan := d.client.Ack()
		nackChan := d.client.Nack()
		var err error
		var uid ulid.ULID

		for queue.WaitManyAckQueues(ackChan, nackChan) {
			for {
				uid, _, err = ackChan.Get()
				if err != nil {
					break
				}
				d.ack(uid, conf.RELP)
				ackCounter.WithLabelValues("relp", "ack", "").Inc()
			}
			for {
				uid, _, err = nackChan.Get()
				if err != nil {
					break
				}
				d.nack(uid, conf.RELP)
				d.logger.Info("RELP server returned a NACK", "uid", uid.String())
				ackCounter.WithLabelValues("relp", "nack", "").Inc()
				fatalCounter.WithLabelValues("relp").Inc()
				d.once.Do(func() { close(d.fatal) })
			}
		}
	}()

	return d, nil
}

func (d *relpDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.client.Send(&message)
	if err != nil {
		// the client send queue has been disposed
		ackCounter.WithLabelValues("relp", "nack", "").Inc()
		fatalCounter.WithLabelValues("relp").Inc()
		d.nack(message.Uid, conf.RELP)
		d.once.Do(func() { close(d.fatal) })
	}
	return
}

func (d *relpDestination) Close() (err error) {
	return d.client.Close()
}

func (d *relpDestination) Fatal() chan struct{} {
	return d.fatal
}
