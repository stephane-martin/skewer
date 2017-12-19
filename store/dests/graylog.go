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

// TODO: metrics

type graylogDestination struct {
	logger  log15.Logger
	fatal   chan struct{}
	once    sync.Once
	ack     storeCallback
	nack    storeCallback
	permerr storeCallback
	writer  gelf.Writer
}

func NewGraylogDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	hostport := net.JoinHostPort(bc.GraylogDest.Host, strconv.FormatInt(int64(bc.GraylogDest.Port), 10))
	var w gelf.Writer
	if strings.ToLower(strings.TrimSpace(bc.GraylogDest.Mode)) == "udp" {
		writer, err := gelf.NewUDPWriter(hostport)
		if err != nil {
			return nil, err
		}
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
			return nil, err
		}
		writer.MaxReconnect = bc.GraylogDest.MaxReconnect
		writer.ReconnectDelay = bc.GraylogDest.ReconnectDelay
		w = writer
	}

	d := &graylogDestination{
		logger:  logger,
		fatal:   make(chan struct{}),
		ack:     ack,
		nack:    nack,
		permerr: permerr,
		writer:  w,
	}
	return d, nil
}

func (d *graylogDestination) Close() error {
	return d.writer.Close()
}

func (d *graylogDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *graylogDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	err = d.writer.WriteMessage(message.Parsed.Fields.ToGelfMessage())
	if err == nil {
		d.ack(message.Uid, conf.Graylog)
	} else {
		d.nack(message.Uid, conf.Graylog)
		d.once.Do(func() { close(d.fatal) })
	}
	return err
}
