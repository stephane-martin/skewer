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
	delimiter   []byte
	previous    *model.TcpUdpParsedMessage
}

func NewTcpDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	d := &tcpDestination{
		logger:      logger,
		registry:    prometheus.NewRegistry(),
		ack:         ack,
		nack:        nack,
		permerr:     permerr,
		format:      bc.TcpDest.Format,
		lineFraming: bc.TcpDest.LineFraming,
		delimiter:   []byte{bc.TcpDest.FrameDelimiter},
	}

	defer func() {
		if d.conn != nil && err != nil {
			d.conn.Close()
		}
	}()

	//d.registry.MustRegister(d.ackCounter)

	d.fatal = make(chan struct{})

	path := strings.TrimSpace(bc.TcpDest.UnixSocketPath)
	if len(path) == 0 {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", bc.TcpDest.Host, bc.TcpDest.Port))
		if err != nil {
			logger.Error("Error resolving TCP address", "error", err)
			return nil, err
		}
		tcpconn, err := net.DialTCP("tcp", nil, addr)
		if err != nil {
			logger.Error("Error connecting on TCP", "error", err)
			return nil, err
		}
		tcpconn.SetNoDelay(true)
		if bc.TcpDest.KeepAlive {
			tcpconn.SetKeepAlive(true)
			tcpconn.SetKeepAlivePeriod(bc.TcpDest.KeepAlivePeriod)
		}
		d.conn = tcpconn
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

func (d *tcpDestination) Send(message *model.TcpUdpParsedMessage, partitionKey string, partitionNumber int32, topic string) error {
	serial, err := message.MarshalAll(d.format)
	if err != nil {
		d.permerr(message.Uid)
		return err
	}
	if d.lineFraming {
		err = utils.ChainWrites(d.conn, serial, d.delimiter)
	} else {
		err = utils.ChainWrites(
			d.conn,
			[]byte(strconv.FormatInt(int64(len(serial)), 10)),
			sp,
			serial,
		)
	}
	if err == nil {
		if d.previous != nil {
			d.ack(d.previous.Uid)
		}
		d.previous = message
	} else {
		d.nack(message.Uid)
		if d.previous != nil {
			d.nack(d.previous.Uid)
			d.previous = nil
		}
		d.once.Do(func() { close(d.fatal) })
		return err
	}
	return nil
}

func (d *tcpDestination) Close() {
	d.conn.Close()
}

func (d *tcpDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *tcpDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}
