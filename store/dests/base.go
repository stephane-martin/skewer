package dests

import (
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
	fatal    chan struct{}
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
		fatal:    make(chan struct{}),
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

func (base *baseDestination) NACKAll(msgs []*model.FullMessage) {
	for _, msg := range msgs {
		base.ACK(msg.Uid)
	}
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

func (base *baseDestination) Fatal() chan struct{} {
	return base.fatal
}

func (base *baseDestination) dofatal() {
	base.once.Do(func() {
		fatalCounter.WithLabelValues(base.codename).Inc()
		close(base.fatal)
	})
}

// TODO: destinations should free the given syslog message when they are sure they dont need it anymore
