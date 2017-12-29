package dests

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type GraylogDestination struct {
	logger  log15.Logger
	fatal   chan struct{}
	once    sync.Once
	ack     storeCallback
	nack    storeCallback
	permerr storeCallback
	writer  gelf.Writer
}

func NewGraylogDestination(ctx context.Context, cfnd bool, bc conf.BaseConfig, ack, nack, pe storeCallback, l log15.Logger) (d *GraylogDestination, err error) {
	hostport := net.JoinHostPort(bc.GraylogDest.Host, strconv.FormatInt(int64(bc.GraylogDest.Port), 10))
	var w gelf.Writer
	if strings.ToLower(strings.TrimSpace(bc.GraylogDest.Mode)) == "udp" {
		writer, err := gelf.NewUDPWriter(hostport)
		if err != nil {
			connCounter.WithLabelValues("graylog", "fail").Inc()
			return nil, err
		}
		connCounter.WithLabelValues("graylog", "success").Inc()
		writer.CompressionLevel = bc.GraylogDest.CompressionLevel
		switch strings.TrimSpace(strings.ToLower(bc.GraylogDest.CompressionType)) {
		case "gzip":
			writer.CompressionType = gelf.CompressGzip
		case "zlib":
			writer.CompressionType = gelf.CompressZlib
		case "none", "":
			writer.CompressionType = gelf.CompressNone
		default:
			writer.CompressionType = gelf.CompressGzip
		}
		w = writer
	} else {
		writer, err := gelf.NewTCPWriter(hostport)
		if err != nil {
			connCounter.WithLabelValues("graylog", "fail").Inc()
			return nil, err
		}
		connCounter.WithLabelValues("graylog", "success").Inc()
		writer.MaxReconnect = bc.GraylogDest.MaxReconnect
		writer.ReconnectDelay = bc.GraylogDest.ReconnectDelay
		w = writer
	}

	d = &GraylogDestination{
		logger:  l,
		fatal:   make(chan struct{}),
		ack:     ack,
		nack:    nack,
		permerr: pe,
		writer:  w,
	}
	return d, nil
}

func (d *GraylogDestination) Close() error {
	return d.writer.Close()
}

func (d *GraylogDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *GraylogDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.writer.WriteMessage(message.ToGelfMessage())
	if err == nil {
		d.ack(message.Uid, conf.Graylog)
		ackCounter.WithLabelValues("graylog", "ack", "").Inc()
	} else {
		d.nack(message.Uid, conf.Graylog)
		ackCounter.WithLabelValues("graylog", "nack", "").Inc()
		fatalCounter.WithLabelValues("graylog").Inc()
		d.once.Do(func() { close(d.fatal) })
	}
	return err
}
