package dests

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
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
	var buf string
	buf, err = encoders.ChainEncode(d.encoder, message, "\n")
	if err != nil {
		return err
	}
	_, err = io.WriteString(os.Stderr, buf)
	return err
}

func (d *StderrDestination) Close() error {
	return nil
}

func (d *StderrDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var msg *model.FullMessage
	var uid utils.MyULID
	var e error
	for len(msgs) > 0 {
		msg = msgs[0].Message
		uid = msg.Uid
		msgs = msgs[1:]
		e = d.sendOne(msg)
		model.FullFree(msg)
		if e != nil {
			if encoders.IsEncodingError(e) {
				d.PermError(uid)
			} else {
				d.NACK(uid)
				d.NACKRemaining(msgs)
				d.dofatal()
				return e
			}
			if err == nil {
				err = e
			}
		} else {
			d.ACK(uid)
		}
	}
	return err
}
