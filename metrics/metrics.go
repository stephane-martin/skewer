package metrics

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stephane-martin/skewer/conf"
)

type MetricsServer struct {
	/*
		BadgerGauge                 *prometheus.GaugeVec
		IncomingMsgsCounter         *prometheus.CounterVec
		ClientConnectionCounter     *prometheus.CounterVec
		ParsingErrorCounter         *prometheus.CounterVec
		RelpAnswersCounter          *prometheus.CounterVec
		RelpProtocolErrorsCounter   *prometheus.CounterVec
		KafkaConnectionErrorCounter prometheus.Counter
		KafkaAckNackCounter         *prometheus.CounterVec
		MessageFilteringCounter     *prometheus.CounterVec
		Registry                    *prometheus.Registry
	*/
	server *http.Server
}

/*
func SetupMetrics(c conf.MetricsConfig) *Metrics {
	m := Metrics{}

	m.BadgerGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "skw_badger_entries_gauge",
			Help: "number of messages stored in the badger database",
		},
		[]string{"partition"},
	)

	m.IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_incoming_messages_total",
			Help: "total number of syslog messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)

	m.ClientConnectionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_client_connections_total",
			Help: "total number of client connections",
		},
		[]string{"protocol", "client", "port", "path"},
	)

	m.ParsingErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_parsing_errors_total",
			Help: "total number of times there was a parsing error",
		},
		[]string{"parser_name", "client"},
	)

	m.RelpAnswersCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relp_answers_total",
			Help: "number of RELP rsp answers",
		},
		[]string{"status", "client"},
	)

	m.RelpProtocolErrorsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relp_protocol_errors_total",
			Help: "Number of RELP protocol errors",
		},
		[]string{"client"},
	)

	m.KafkaConnectionErrorCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "skw_kafka_connection_errors_total",
			Help: "number of kafka connection errors",
		},
	)

	m.KafkaAckNackCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_kafka_ack_total",
			Help: "number of kafka acknowledgments",
		},
		[]string{"status", "topic"},
	)

	m.MessageFilteringCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_messages_filtering_total",
			Help: "number of filtered messages by status",
		},
		[]string{"status", "client"},
	)

	m.Registry = prometheus.NewRegistry()
	m.Registry.MustRegister(
		m.BadgerGauge,
		m.IncomingMsgsCounter,
		m.ClientConnectionCounter,
		m.ParsingErrorCounter,
		m.RelpAnswersCounter,
		m.RelpProtocolErrorsCounter,
		m.KafkaConnectionErrorCounter,
		m.KafkaAckNackCounter,
		m.MessageFilteringCounter,
	)

	m.NewConf(c)
	return &m
}
*/

func (m *MetricsServer) Stop() {
	if m.server != nil {
		m.server.Close()
		m.server = nil
	}
}

func (m *MetricsServer) NewConf(c conf.MetricsConfig, gatherers ...prometheus.Gatherer) {
	m.Stop()
	var nonNilGatherers prometheus.Gatherers = []prometheus.Gatherer{}
	for _, gatherer := range gatherers {
		if gatherer != nil {
			nonNilGatherers = append(nonNilGatherers, gatherer)
		}
	}

	if strings.TrimSpace(c.Path) == "" {
		c.Path = "/metrics"
	}
	if c.Port > 0 {
		mux := http.NewServeMux()
		mux.Handle(c.Path, promhttp.HandlerFor(nonNilGatherers, promhttp.HandlerOpts{}))
		m.server = &http.Server{
			Addr:    fmt.Sprintf("127.0.0.1:%d", c.Port),
			Handler: mux,
		}

		go func() {
			// actually listen
			m.server.ListenAndServe()
		}()
	}
}
