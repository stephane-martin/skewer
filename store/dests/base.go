package dests

import (
	"context"
	"fmt"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/encoders/baseenc"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/queue/message"
)

var Registry *prometheus.Registry

var ackCounter *prometheus.CounterVec
var connCounter *prometheus.CounterVec
var fatalCounter *prometheus.CounterVec
var httpStatusCounter *prometheus.CounterVec
var kafkaInputsCounter prometheus.Counter
var openedFilesGauge prometheus.Gauge

var once sync.Once

func InitRegistry() {
	once.Do(func() {
		ackCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_dest_ack_total",
				Help: "number of message acknowledgments",
			},
			[]string{"dest", "status"},
		)

		connCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_dest_conn_total",
				Help: "number of connections to remote service",
			},
			[]string{"dest", "status"},
		)

		fatalCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_dest_fatal_total",
				Help: "number of destination fatal errors",
			},
			[]string{"dest"},
		)

		httpStatusCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_http_status_total",
				Help: "number of returned status codes for HTTP destination",
			},
			[]string{"host", "code"},
		)

		kafkaInputsCounter = prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "skw_dest_kafka_sent_total",
				Help: "number of sent messages to kafka",
			},
		)

		openedFilesGauge = prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "skw_dest_opened_files_number",
				Help: "number of opened files by the file destination",
			},
		)

		Registry = prometheus.NewRegistry()
		Registry.MustRegister(
			ackCounter,
			connCounter,
			fatalCounter,
			kafkaInputsCounter,
			httpStatusCounter,
			openedFilesGauge,
		)
	})
}

type Env struct {
	logger   log15.Logger
	binder   binder.Client
	ack      storeCallback
	nack     storeCallback
	permerr  storeCallback
	confined bool
	config   conf.BaseConfig
}

func BuildEnv() *Env {
	return &Env{}
}

func (e *Env) Binder(b binder.Client) *Env {
	e.binder = b
	return e
}

func (e *Env) Logger(l log15.Logger) *Env {
	e.logger = l
	return e
}

func (e *Env) Callbacks(a, n, p storeCallback) *Env {
	e.ack = a
	e.nack = n
	e.permerr = p
	return e
}

func (e *Env) Confined(c bool) *Env {
	e.confined = c
	return e
}

func (e *Env) Config(c conf.BaseConfig) *Env {
	e.config = c
	return e
}

type baseDestination struct {
	logger   log15.Logger
	binder   binder.Client
	fatal    chan error
	once     *sync.Once
	sack     storeCallback
	snack    storeCallback
	spermerr storeCallback
	confined bool
	format   baseenc.Format
	encoder  encoders.Encoder
	codename string
	typ      conf.DestinationType
}

func newBaseDestination(typ conf.DestinationType, codename string, e *Env) *baseDestination {
	base := baseDestination{
		logger:   e.logger,
		binder:   e.binder,
		fatal:    make(chan error),
		once:     &sync.Once{},
		confined: e.confined,
		codename: codename,
		typ:      typ,
		sack:     e.ack,
		snack:    e.nack,
		spermerr: e.permerr,
	}
	return &base
}

func (base *baseDestination) ACK(uid utils.MyULID) {
	base.sack(uid, base.typ)
	ackCounter.WithLabelValues(base.codename, "ack").Inc()
}

func (base *baseDestination) NACK(uid utils.MyULID) {
	base.snack(uid, base.typ)
	ackCounter.WithLabelValues(base.codename, "nack").Inc()
}

func (base *baseDestination) PermError(uid utils.MyULID) {
	base.spermerr(uid, base.typ)
	ackCounter.WithLabelValues(base.codename, "permerr").Inc()
}

func (base *baseDestination) NACKAll(msgQ *message.Ring) {
	msgQ.Dispose()
	var msg *model.FullMessage
	var err error
	for {
		msg, err = msgQ.Get()
		if err != nil || msg == nil {
			break
		}
		base.NACK(msg.Uid)
		model.FullFree(msg)
	}
}

func (base *baseDestination) NACKAllSlice(msgs []*model.FullMessage) {
	var msg *model.FullMessage
	for _, msg = range msgs {
		base.NACK(msg.Uid)
		model.FullFree(msg)
	}
}

func (base *baseDestination) NACKRemaining(msgs []model.OutputMsg) {
	for i := range msgs {
		base.NACK(msgs[i].Message.Uid)
		model.FullFree(msgs[i].Message)
	}
}

func (base *baseDestination) ForEach(ctx context.Context, f func(context.Context, *model.FullMessage) error, ackf, free bool, msgs []model.OutputMsg) (err eerrors.ErrorSlice) {
	var msg *model.FullMessage
	var curErr error
	c := eerrors.ChainErrors()
	var uid utils.MyULID
	for len(msgs) > 0 {
		msg = msgs[0].Message
		uid = msg.Uid
		curErr = f(ctx, msg)
		msgs = msgs[1:]
		if free {
			model.FullFree(msg)
		}
		if curErr != nil {
			c.Append(curErr)
			if IsEncodingError(curErr) {
				base.PermError(uid)
			} else {
				base.NACK(uid)
				base.NACKRemaining(msgs)
				base.dofatal(curErr)
				return c.Sum()
			}
		} else if ackf {
			base.ACK(uid)
		}
	}
	return c.Sum()
}

func (base *baseDestination) ForEachWithTopic(ctx context.Context, f func(context.Context, *model.FullMessage, string, string, int32) error, ackf, free bool, msgs []model.OutputMsg) (err eerrors.ErrorSlice) {
	var msg *model.FullMessage
	var curErr error
	c := eerrors.ChainErrors()
	var uid utils.MyULID
	for len(msgs) > 0 {
		msg = msgs[0].Message
		uid = msg.Uid
		curErr = f(ctx, msg, msgs[0].Topic, msgs[0].PartitionKey, msgs[0].PartitionNumber)
		msgs = msgs[1:]
		if free {
			model.FullFree(msg)
		}
		if curErr != nil {
			c.Append(curErr)
			if IsEncodingError(curErr) {
				base.PermError(uid)
			} else {
				base.NACK(uid)
				base.NACKRemaining(msgs)
				base.dofatal(curErr)
				return c.Sum()
			}
		} else if ackf {
			base.ACK(uid)
		}
	}
	return c.Sum()
}

func (base *baseDestination) getEncoder(format string) (frmt baseenc.Format, encoder encoders.Encoder, err error) {
	frmt = baseenc.ParseFormat(format)
	if frmt == -1 {
		return 0, nil, fmt.Errorf("Unknown encoding format: %s", format)
	}
	encoder, err = encoders.GetEncoder(frmt)
	if err != nil {
		return 0, nil, err
	}
	return frmt, encoder, nil
}

func (base *baseDestination) setFormat(format string) error {
	frmt, encoder, err := base.getEncoder(format)
	if err != nil {
		return err
	}
	base.format = frmt
	base.encoder = encoder
	return nil
}

func (base *baseDestination) Fatal() chan error {
	return base.fatal
}

func (base *baseDestination) dofatal(err error) {
	base.once.Do(func() {
		fatalCounter.WithLabelValues(base.codename).Inc()
		base.fatal <- eerrors.Fatal(eerrors.Wrapf(err, "Fatal error happened in destination '%s'", base.codename))
		close(base.fatal)
	})
}

// IsEncodingError returns true when the given error is a message encoding error
func IsEncodingError(err error) bool {
	if err == nil {
		return false
	}
	return eerrors.Is("Encoding", err)
}

// TODO: destinations should free the given syslog message when they are sure they dont need it anymore
