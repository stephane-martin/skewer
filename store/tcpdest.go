package store

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

var sp = []byte(" ")
var endl = []byte("\n")

type tcpDestination struct {
	logger      log15.Logger
	fatal       chan struct{}
	registry    *prometheus.Registry
	conn        net.Conn
	once        sync.Once
	format      string
	ack         storeCallback
	nack        storeCallback
	permerr     storeCallback
	lineFraming bool
}

func NewTcpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (Destination, error) {
	d := &tcpDestination{
		logger:      logger,
		registry:    prometheus.NewRegistry(),
		ack:         ack,
		nack:        nack,
		permerr:     permerr,
		format:      bc.TcpDest.Format,
		lineFraming: bc.TcpDest.LineFraming,
	}

	//d.registry.MustRegister(d.ackCounter)

	d.fatal = make(chan struct{})

	path := strings.TrimSpace(bc.TcpDest.UnixSocketPath)
	if len(path) == 0 {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", bc.TcpDest.Host, bc.TcpDest.Port))
		if err != nil {
			logger.Error("Error resolving TCP address", "error", err)
			return nil, err
		}

		d.conn, err = net.DialTCP("tcp", nil, addr)
		if err != nil {
			logger.Error("Error connecting on TCP", "error", err)
			return nil, err
		}
	} else {
		addr, err := net.ResolveUnixAddr("unix", path)
		if err != nil {
			logger.Error("Error resolving unix socket path", "error", err)
			return nil, err
		}

		d.conn, err = net.DialUnix("unix", nil, addr)
		if err != nil {
			logger.Error("Error connecting to unix socket", "error", err)
			return nil, err
		}
	}

	rebind := bc.TcpDest.Rebind
	if rebind > 0 {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(rebind):
				logger.Info("TCP destination rebind period has expired", "rebind", rebind.String())
				d.once.Do(func() { close(d.fatal) })
			}
		}()
	}

	return d, nil
}

func (d *tcpDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}

func (d *tcpDestination) Send(message *model.TcpUdpParsedMessage, partitionKey string, partitionNumber int32, topic string) error {
	serial, err := message.MarshalAll(d.format)
	if err != nil {
		d.permerr(message.Uid)
		return err
	}
	if !d.lineFraming {
		err = utils.ChainWrites(
			d.conn,
			[]byte(strconv.FormatInt(int64(len(serial)), 10)),
			sp,
			serial,
			endl,
		)
	} else {
		err = utils.ChainWrites(
			d.conn,
			serial,
			endl,
		)
	}

	if err != nil {
		d.nack(message.Uid)
		d.once.Do(func() { close(d.fatal) })
		return err
	}
	d.ack(message.Uid)
	return nil
}

func (d *tcpDestination) Close() {
	d.conn.Close()
}

func (d *tcpDestination) Fatal() chan struct{} {
	return d.fatal
}
