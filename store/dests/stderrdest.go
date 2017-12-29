package dests

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type StderrDestination struct {
	logger  log15.Logger
	fatal   chan struct{}
	once    sync.Once
	ack     storeCallback
	nack    storeCallback
	permerr storeCallback
	format  string
	encoder model.Encoder
}

func NewStderrDestination(ctx context.Context, cfnd bool, bc conf.BaseConfig, ack, nack, pe storeCallback, l log15.Logger) (d *StderrDestination, err error) {
	d = &StderrDestination{
		logger:  l,
		ack:     ack,
		nack:    nack,
		permerr: pe,
		format:  bc.StderrDest.Format,
		fatal:   make(chan struct{}),
	}
	d.encoder, err = model.NewEncoder(d.format)
	if err != nil {
		return nil, fmt.Errorf("Error getting encoder: %s", err)
	}

	return d, nil
}

func (d *StderrDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	var buf []byte
	buf, err = model.ChainEncode(d.encoder, &message, "\n")
	if err != nil {
		ackCounter.WithLabelValues("stderr", "permerr", "").Inc()
		d.permerr(message.Uid, conf.Stderr)
		return err
	}
	_, err = os.Stderr.Write(buf)
	if err != nil {
		ackCounter.WithLabelValues("stderr", "nack", "").Inc()
		fatalCounter.WithLabelValues("stderr").Inc()
		d.nack(message.Uid, conf.Stderr)
		d.once.Do(func() { close(d.fatal) })
		return err
	}
	ackCounter.WithLabelValues("stderr", "ack", "").Inc()
	d.ack(message.Uid, conf.Stderr)
	return nil
}

func (d *StderrDestination) Close() error {
	return nil
}

func (d *StderrDestination) Fatal() chan struct{} {
	return d.fatal
}
