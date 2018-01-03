package dests

import (
	"fmt"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model/encoders"
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
				Name: "skw_relpdest_ack_total",
				Help: "number of RELP acknowledgments",
			},
			[]string{"dest", "status"},
		)

		connCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_relpdest_conn_total",
				Help: "number of RELP connections",
			},
			[]string{"dest", "status"},
		)

		fatalCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "skw_kafka_fatal_total",
				Help: "number of received kafka fatal errors",
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
				Name: "skw_kafka_inputs_total",
				Help: "number of sent messages to kafka",
			},
		)

		openedFilesGauge = prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "skw_file_opened_files_number",
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
	ack      storeCallback
	nack     storeCallback
	permerr  storeCallback
	confined bool
	config   conf.BaseConfig
}

func BuildEnv() *Env {
	return &Env{}
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

type callback func(uid utils.MyULID)

type baseDestination struct {
	logger   log15.Logger
	fatal    chan struct{}
	once     *sync.Once
	ack      callback
	nack     callback
	permerr  callback
	confined bool
	format   encoders.Format
	encoder  encoders.Encoder
	codename string
	typ      conf.DestinationType
}

func newBaseDestination(typ conf.DestinationType, codename string, e *Env) *baseDestination {
	base := baseDestination{
		logger:   e.logger,
		fatal:    make(chan struct{}),
		once:     &sync.Once{},
		confined: e.confined,
		codename: codename,
		typ:      typ,
	}
	base.ack = func(uid utils.MyULID) {
		e.ack(uid, typ)
		ackCounter.WithLabelValues(codename, "ack").Inc()
	}
	base.nack = func(uid utils.MyULID) {
		e.nack(uid, typ)
		ackCounter.WithLabelValues(codename, "nack").Inc()
	}
	base.permerr = func(uid utils.MyULID) {
		e.permerr(uid, typ)
		ackCounter.WithLabelValues(codename, "permerr").Inc()
	}
	return &base
}

func (base *baseDestination) setFormat(format string) error {
	frmt := encoders.ParseFormat(format)
	if frmt == -1 {
		return fmt.Errorf("Unknown encoding format: %s", format)
	}
	encoder, err := encoders.GetEncoder(frmt)
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
