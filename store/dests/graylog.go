package dests

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"gopkg.in/Graylog2/go-gelf.v2/gelf"
)

type GraylogDestination struct {
	*baseDestination
	writer gelf.Writer
}

func NewGraylogDestination(ctx context.Context, e *Env) (Destination, error) {
	hostport := net.JoinHostPort(e.config.GraylogDest.Host, strconv.FormatInt(int64(e.config.GraylogDest.Port), 10))
	var w gelf.Writer
	if strings.ToLower(strings.TrimSpace(e.config.GraylogDest.Mode)) == "udp" {
		writer, err := gelf.NewUDPWriter(hostport)
		if err != nil {
			connCounter.WithLabelValues("graylog", "fail").Inc()
			return nil, err
		}
		connCounter.WithLabelValues("graylog", "success").Inc()
		writer.CompressionLevel = e.config.GraylogDest.CompressionLevel
		switch strings.TrimSpace(strings.ToLower(e.config.GraylogDest.CompressionType)) {
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
		writer.MaxReconnect = e.config.GraylogDest.MaxReconnect
		writer.ReconnectDelay = e.config.GraylogDest.ReconnectDelay
		w = writer
	}

	d := &GraylogDestination{
		baseDestination: newBaseDestination(conf.Graylog, "graylog", e),
		writer:          w,
	}
	return d, nil
}

func (d *GraylogDestination) Close() error {
	return d.writer.Close()
}

func (d *GraylogDestination) sendOne(message *model.FullMessage) (err error) {
	err = d.writer.WriteMessage(encoders.FullToGelfMessage(message))
	if err == nil {
		d.ack(message.Uid)
	} else {
		d.nack(message.Uid)
		d.dofatal()
	}
	model.FullFree(message)
	return err
}

func (d *GraylogDestination) Send(msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var i int
	var e error
	for i = range msgs {
		e = d.sendOne(msgs[i].Message)
		if e != nil {
			err = e
		}
	}
	return err
}
