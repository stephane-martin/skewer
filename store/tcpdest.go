package store

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

var sp = []byte(" ")
var zero ulid.ULID

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
	previousUid ulid.ULID
	encoder     model.Encoder
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
		fatal:       make(chan struct{}),
	}

	defer func() {
		if d.conn != nil && err != nil {
			d.conn.Close()
		}
	}()

	//d.registry.MustRegister(d.ackCounter)

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

	d.encoder, err = model.NewEncoder(d.conn, d.format)
	if err != nil {
		return nil, err
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

func (d *tcpDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	if d.lineFraming {
		err = model.ChainEncode(d.encoder, &message, d.delimiter)
	} else {
		err = model.FrameEncode(d.encoder, nil, &message)
	}
	if err == nil {
		if d.previousUid != zero {
			d.ack(d.previousUid)
		}
		d.previousUid = message.Uid
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid)
	} else {
		// error writing to d.conn
		d.nack(message.Uid)
		if d.previousUid != zero {
			d.nack(d.previousUid)
			d.previousUid = zero
		}
		d.once.Do(func() { close(d.fatal) })
	}
	return
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
