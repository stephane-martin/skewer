package base

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var Registry *prometheus.Registry
var Once sync.Once

var IncomingMsgsCounter *prometheus.CounterVec
var ClientConnectionCounter *prometheus.CounterVec
var ParsingErrorCounter *prometheus.CounterVec
var MessageFilteringCounter *prometheus.CounterVec

func InitRegistry() {
	IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_incoming_messages_total",
			Help: "total number of messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)

	ClientConnectionCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_client_connections_total",
			Help: "total number of client connections",
		},
		[]string{"protocol", "client", "port", "path"},
	)

	ParsingErrorCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_parsing_errors_total",
			Help: "total number of times there was a parsing error",
		},
		[]string{"protocol", "client", "parser_name"},
	)

	MessageFilteringCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relp_messages_filtering_total",
			Help: "number of filtered messages by status",
		},
		[]string{"status", "client"},
	)

	Registry = prometheus.NewRegistry()
	Registry.MustRegister(
		ClientConnectionCounter,
		IncomingMsgsCounter,
		MessageFilteringCounter,
		ParsingErrorCounter,
	)
}
