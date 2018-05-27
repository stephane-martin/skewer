package utils

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	metrics "github.com/rcrowley/go-metrics"
)

func KafkaProducerMetrics(mregistry metrics.Registry, basename string) (collectors []prometheus.Collector) {
	collectors = make([]prometheus.Collector, 0)

	collectors = append(collectors, prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Help: "Kafka mean outgoing bytes/s rate",
			Name: fmt.Sprintf(basename + "_outgoing_byte_rate"),
		},
		func() float64 {
			meter := mregistry.Get("outgoing-byte-rate")
			if meter == nil {
				return 0
			}
			return meter.(metrics.Meter).Rate1()
		},
	))

	collectors = append(collectors, prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Help: "Kafka mean outgoing requests/s rate",
			Name: fmt.Sprintf(basename + "_request_rate"),
		},
		func() float64 {
			meter := mregistry.Get("request-rate")
			if meter == nil {
				return 0
			}
			return meter.(metrics.Meter).Rate1()
		},
	))

	collectors = append(collectors, prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Help: "Kafka mean request size",
			Name: fmt.Sprintf(basename + "_request_size"),
		},
		func() float64 {
			meter := mregistry.Get("request-size")
			if meter == nil {
				return 0
			}
			return meter.(metrics.Histogram).Mean()
		},
	))

	collectors = append(collectors, prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Help: "Kafka mean outgoing records/s rate",
			Name: fmt.Sprintf(basename + "_record_rate"),
		},
		func() float64 {
			meter := mregistry.Get("record-send-rate")
			if meter == nil {
				return 0
			}
			return meter.(metrics.Meter).Rate1()
		},
	))

	return collectors
}

func KafkaConsumerMetrics(mregistry metrics.Registry, basename string) (collectors []prometheus.Collector) {
	collectors = make([]prometheus.Collector, 0)

	collectors = append(collectors, prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Help: "Kafka mean incoming byte rate",
			Name: fmt.Sprintf(basename + "_incoming_byte_rate"),
		},
		func() float64 {
			meter := mregistry.Get("incoming-byte-rate")
			if meter == nil {
				return 0
			}
			return meter.(metrics.Meter).Rate1()
		},
	))

	collectors = append(collectors, prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Help: "Kafka mean responses/s rate",
			Name: fmt.Sprintf(basename + "_response_rate"),
		},
		func() float64 {
			meter := mregistry.Get("response-rate")
			if meter == nil {
				return 0
			}
			return meter.(metrics.Meter).Rate1()
		},
	))

	collectors = append(collectors, prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Help: "Kafka mean response size",
			Name: fmt.Sprintf(basename + "_response_size"),
		},
		func() float64 {
			meter := mregistry.Get("response-size")
			if meter == nil {
				return 0
			}
			return meter.(metrics.Histogram).Mean()
		},
	))

	return collectors
}
