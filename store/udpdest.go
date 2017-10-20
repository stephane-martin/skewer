package store

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type udpDestination struct {
	logger   log15.Logger
	succ     chan ulid.ULID
	fail     chan ulid.ULID
	fatal    chan struct{}
	registry *prometheus.Registry
	conn     *net.UDPConn
	once     sync.Once
}

func NewUdpDestination(ctx context.Context, bc conf.BaseConfig, logger log15.Logger) Destination {
	d := &udpDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
	}
	//d.registry.MustRegister(d.ackCounter)
	d.succ = make(chan ulid.ULID, 100)
	d.fail = make(chan ulid.ULID, 100)
	close(d.fail)
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

func (d *udpDestination) Send(message *model.TcpUdpParsedMessage, partitionKey string, partitionNumber int32, topic string) (bool, error) {
	serial, err := message.Parsed.Fields.Marshal5424()
	if err != nil {
		return true, err
	}
	serial = append(serial, '\n')
	_, err = d.conn.Write(serial)
	if err != nil {
		d.once.Do(func() { close(d.fatal) })
		return false, err
	}
	d.succ <- message.Uid
	return false, nil
}

func (d *udpDestination) Close() {
	d.conn.Close()
	close(d.succ)
}

func (d *udpDestination) Successes() chan ulid.ULID {
	return d.succ
}

func (d *udpDestination) Failures() chan ulid.ULID {
	return d.fail
}

func (d *udpDestination) Fatal() chan struct{} {
	return d.fatal
}
