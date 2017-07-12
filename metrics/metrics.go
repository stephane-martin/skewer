package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	IncomingMsgsCounter         *prometheus.CounterVec
	ClientConnectionCounter     *prometheus.CounterVec
	ParsingErrorCounter         *prometheus.CounterVec
	RelpAnswersCounter          *prometheus.CounterVec
	RelpProtocolErrorsCounter   *prometheus.CounterVec
	KafkaConnectionErrorCounter prometheus.Counter
	KafkaAckNackCounter         *prometheus.CounterVec
	MessageFilteringCounter     *prometheus.CounterVec
}

func SetupMetrics() *Metrics {
	m := Metrics{}

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

	prometheus.MustRegister(m.IncomingMsgsCounter)
	prometheus.MustRegister(m.ClientConnectionCounter)
	prometheus.MustRegister(m.ParsingErrorCounter)
	prometheus.MustRegister(m.RelpAnswersCounter)
	prometheus.MustRegister(m.RelpProtocolErrorsCounter)
	prometheus.MustRegister(m.KafkaConnectionErrorCounter)
	prometheus.MustRegister(m.KafkaAckNackCounter)
	prometheus.MustRegister(m.MessageFilteringCounter)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: mux,
	}
	mux.Handle("/metrics", promhttp.Handler())

	go func() {
		server.ListenAndServe()
	}()

	return &m
}
