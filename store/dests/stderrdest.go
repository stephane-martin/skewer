package dests

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils/eerrors"
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

func (d *StderrDestination) sendOne(ctx context.Context, message *model.FullMessage) (err error) {
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

func (d *StderrDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err eerrors.ErrorSlice) {
	return d.ForEach(ctx, d.sendOne, true, true, msgs)
}
