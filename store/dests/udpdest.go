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

// TODO: metrics

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
		Host(bc.UdpDest.Host).
		Port(bc.UdpDest.Port).
		Path(bc.UdpDest.UnixSocketPath).
		Format(bc.UdpDest.Format)

	err = client.Connect()
	if err != nil {
		return nil, err
	}

	d := &udpDestination{
		logger:  logger,
		ack:     ack,
		nack:    nack,
		permerr: permerr,
		fatal:   make(chan struct{}),
		client:  client,
	}

	rebind := bc.UdpDest.Rebind
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
		d.ack(message.Uid, conf.Udp)
		return nil
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid, conf.Udp)
		return err
	} else {
		// error writing to the UDP conn
		d.nack(message.Uid, conf.Udp)
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
