package store

import (
	"context"
	"fmt"
	"net"
	"sync"

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
	conn     *net.UDPConn
	once     sync.Once
	ack      storeCallback
	nack     storeCallback
	permerr  storeCallback
}

func NewUdpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) Destination {
	d := &udpDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
	}
	//d.registry.MustRegister(d.ackCounter)
	d.fatal = make(chan struct{})
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", bc.UdpDest.Host, bc.UdpDest.Port))
	if err != nil {
		logger.Error("Error resolving UDP address", "error", err)
		return nil
	}
	d.conn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		logger.Error("Error dialing UDP", "error", err)
		return nil
	}

	return d
}

func (d *udpDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}

func (d *udpDestination) Send(message *model.TcpUdpParsedMessage, partitionKey string, partitionNumber int32, topic string) error {
	serial, err := message.Parsed.Fields.Marshal5424()
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
