package dests

import (
	"context"
	"fmt"
	"os"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
)

type StderrDestination struct {
	*baseDestination
}

func NewStderrDestination(ctx context.Context, e *Env) (Destination, error) {
	d := &StderrDestination{
		baseDestination: newBaseDestination(conf.Stderr, "stderr", e),
	}
	err := d.setFormat(e.config.StderrDest.Format)
	if err != nil {
		return nil, fmt.Errorf("Error getting encoder: %s", err)
	}

	return d, nil
}

func (d *StderrDestination) sendOne(message *model.FullMessage) (err error) {
	defer model.FullFree(message)
	var buf []byte
	buf, err = encoders.ChainEncode(d.encoder, message, "\n")
	if err != nil {
		d.permerr(message.Uid)
		return err
	}
	_, err = os.Stderr.Write(buf)
	if err != nil {
		d.nack(message.Uid)
		d.dofatal()
		return err
	}
	d.ack(message.Uid)
	return nil
}

func (d *StderrDestination) Close() error {
	return nil
}

func (d *StderrDestination) Send(msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
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
