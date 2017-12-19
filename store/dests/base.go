package dests

import "github.com/prometheus/client_golang/prometheus"

var DestsRegistry *prometheus.Registry
var relpAckCounter *prometheus.CounterVec
var kafkaAckCounter *prometheus.CounterVec

func init() {
	DestsRegistry = prometheus.NewRegistry()
	relpAckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_relpdest_ack_total",
			Help: "number of RELP acknowledgments",
		},
		[]string{"status"},
	)
	kafkaAckCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_kafkadest_ack_total",
			Help: "number of kafka acknowledgments",
		},
		[]string{"status", "topic"},
	)

	DestsRegistry.MustRegister(relpAckCounter)
	DestsRegistry.MustRegister(kafkaAckCounter)
}
