package dests

import "github.com/prometheus/client_golang/prometheus"

var Registry *prometheus.Registry

var ackCounter *prometheus.CounterVec
var connCounter *prometheus.CounterVec
var fatalCounter *prometheus.CounterVec
var httpStatusCounter *prometheus.CounterVec
var kafkaInputsCounter prometheus.Counter
var openedFilesGauge prometheus.Gauge

func init() {
	Registry = prometheus.NewRegistry()

	ackCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relpdest_ack_total",
			Help: "number of RELP acknowledgments",
		},
		[]string{"dest", "status", "topic"},
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

	Registry.MustRegister(
		ackCounter,
		connCounter,
		fatalCounter,
		kafkaInputsCounter,
		httpStatusCounter,
		openedFilesGauge,
	)
}
