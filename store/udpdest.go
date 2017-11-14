package store

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type udpDestination struct {
	logger   log15.Logger
	fatal    chan struct{}
	registry *prometheus.Registry
	conn     net.Conn
	once     sync.Once
	format   string
	ack      storeCallback
	nack     storeCallback
	permerr  storeCallback
	encoder  model.Encoder
}

func NewUdpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	d := &udpDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
		format:   bc.UdpDest.Format,
		fatal:    make(chan struct{}),
	}

	//d.registry.MustRegister(d.ackCounter)

	defer func() {
		if d.conn != nil && err != nil {
			d.conn.Close()
		}
	}()

	path := strings.TrimSpace(bc.UdpDest.UnixSocketPath)
	if len(path) == 0 {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", bc.UdpDest.Host, bc.UdpDest.Port))
		if err != nil {
			logger.Error("Error resolving UDP address", "error", err)
			return nil, err
		}

		d.conn, err = net.DialUDP("udp", nil, addr)
		if err != nil {
			logger.Error("Error connecting on UDP", "error", err)
			return nil, err
		}
	} else {
		addr, err := net.ResolveUnixAddr("unixgram", path)
		if err != nil {
			logger.Error("Error resolving unix socket path", "error", err)
			return nil, err
		}

		d.conn, err = net.DialUnix("unixgram", nil, addr)
		if err != nil {
			logger.Error("Error connecting to unix socket", "error", err)
			return nil, err
		}
	}

	d.encoder, err = model.NewEncoder(d.conn, d.format)
	if err != nil {
		return nil, err
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

func (d *udpDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}

func (d *udpDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.encoder.Encode(&message)
	if err == nil {
		d.ack(message.Uid, conf.Udp)
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid, conf.Udp)
	} else {
		d.nack(message.Uid, conf.Udp)
		d.once.Do(func() { close(d.fatal) })
	}
	return
}

func (d *udpDestination) Close() {
	d.conn.Close()
}

func (d *udpDestination) Fatal() chan struct{} {
	return d.fatal
}
