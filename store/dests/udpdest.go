package dests

import (
	"context"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/clients"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type udpDestination struct {
	logger  log15.Logger
	fatal   chan struct{}
	once    sync.Once
	ack     storeCallback
	nack    storeCallback
	permerr storeCallback
	client  *clients.SyslogUDPClient
}

func NewUdpDestination(ctx context.Context, confined bool, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	client := clients.NewSyslogUDPClient(logger).
		Host(bc.UDPDest.Host).
		Port(bc.UDPDest.Port).
		Path(bc.UDPDest.UnixSocketPath).
		Format(bc.UDPDest.Format)

	err = client.Connect()
	if err != nil {
		connCounter.WithLabelValues("udp", "fail").Inc()
		return nil, err
	}
	connCounter.WithLabelValues("udp", "success").Inc()

	d := &udpDestination{
		logger:  logger,
		ack:     ack,
		nack:    nack,
		permerr: permerr,
		fatal:   make(chan struct{}),
		client:  client,
	}

	rebind := bc.UDPDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(rebind):
				logger.Info("UDP destination rebind period has expired", "rebind", rebind.String())
				d.once.Do(func() { close(d.fatal) })
			}
		}()
	}

	return d, nil
}

func (d *udpDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.client.Send(&message)
	if err == nil {
		ackCounter.WithLabelValues("udp", "ack", "").Inc()
		d.ack(message.Uid, conf.UDP)
		return nil
	} else if model.IsEncodingError(err) {
		ackCounter.WithLabelValues("udp", "permerr", "").Inc()
		d.permerr(message.Uid, conf.UDP)
		return err
	} else {
		// error writing to the UDP conn
		ackCounter.WithLabelValues("udp", "nack", "").Inc()
		fatalCounter.WithLabelValues("udp").Inc()
		d.nack(message.Uid, conf.UDP)
		d.once.Do(func() { close(d.fatal) })
		return err
	}
}

func (d *udpDestination) Close() error {
	return d.client.Close()
}

func (d *udpDestination) Fatal() chan struct{} {
	return d.fatal
}
