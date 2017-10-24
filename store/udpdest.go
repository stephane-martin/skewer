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
}

func NewUdpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (Destination, error) {
	d := &udpDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
		format:   bc.UdpDest.Format,
	}

	//d.registry.MustRegister(d.ackCounter)

	d.fatal = make(chan struct{})

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

func (d *udpDestination) Send(message *model.TcpUdpParsedMessage, partitionKey string, partitionNumber int32, topic string) error {
	serial, err := message.Parsed.Fields.MarshalAll(d.format)
	if err != nil {
		d.permerr(message.Uid)
		return err
	}
	serial = append(serial, '\n')
	_, err = d.conn.Write(serial)
	if err != nil {
		d.nack(message.Uid)
		d.once.Do(func() { close(d.fatal) })
		return err
	}
	d.ack(message.Uid)
	return nil
}

func (d *udpDestination) Close() {
	d.conn.Close()
}

func (d *udpDestination) Fatal() chan struct{} {
	return d.fatal
}
