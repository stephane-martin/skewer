package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stephane-martin/skewer/conf"
)

type Metrics struct {
	BadgerGauge                 *prometheus.GaugeVec
	IncomingMsgsCounter         *prometheus.CounterVec
	ClientConnectionCounter     *prometheus.CounterVec
	ParsingErrorCounter         *prometheus.CounterVec
	RelpAnswersCounter          *prometheus.CounterVec
	RelpProtocolErrorsCounter   *prometheus.CounterVec
	KafkaConnectionErrorCounter prometheus.Counter
	KafkaAckNackCounter         *prometheus.CounterVec
	MessageFilteringCounter     *prometheus.CounterVec
	server                      *http.Server
}

func SetupMetrics(c conf.MetricsConfig) *Metrics {
	m := Metrics{}

	m.BadgerGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "badger_entries_gauge",
			Help: "number of messages stored in the badger database",
		},
		[]string{"partition"},
	)

	m.IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "incoming_messages_total",
			Help: "total number of syslog messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)

	m.ClientConnectionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "client_connections_total",
			Help: "total number of client connections",
		},
		[]string{"protocol", "client", "port", "path"},
	)

	m.ParsingErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "parsing_errors_total",
			Help: "total number of times there was a parsing error",
		},
		[]string{"parser_name", "client"},
	)

	m.RelpAnswersCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relp_answers_total",
			Help: "number of RELP rsp answers",
		},
		[]string{"status", "client"},
	)

	m.RelpProtocolErrorsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "relp_protocol_errors_total",
			Help: "Number of RELP protocol errors",
		},
		[]string{"client"},
	)

	m.KafkaConnectionErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kafka_connection_errors_total",
			Help: "number of kafka connection errors",
		},
	)

	m.KafkaAckNackCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_ack_total",
			Help: "number of kafka acknowledgments",
		},
		[]string{"status", "topic"},
	)

	m.MessageFilteringCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "messages_filtering_total",
			Help: "number of filtered messages by status",
		},
		[]string{"status", "client"},
	)

	prometheus.MustRegister(m.BadgerGauge)
	prometheus.MustRegister(m.IncomingMsgsCounter)
	prometheus.MustRegister(m.ClientConnectionCounter)
	prometheus.MustRegister(m.ParsingErrorCounter)
	prometheus.MustRegister(m.RelpAnswersCounter)
	prometheus.MustRegister(m.RelpProtocolErrorsCounter)
	prometheus.MustRegister(m.KafkaConnectionErrorCounter)
	prometheus.MustRegister(m.KafkaAckNackCounter)
	prometheus.MustRegister(m.MessageFilteringCounter)

	m.NewConf(c)
	return &m
}

func (m *Metrics) NewConf(c conf.MetricsConfig) {
	if m.server != nil {
		m.server.Close()
	}
	if c.Enabled {
		mux := http.NewServeMux()
		mux.Handle(c.Path, promhttp.Handler())
		m.server = &http.Server{
			Addr:    fmt.Sprintf("127.0.0.1:%d", c.Port),
			Handler: mux,
		}

		go func() {
			m.server.ListenAndServe()
		}()
	}
}
