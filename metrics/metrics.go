package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	IncomingMsgsCounter     *prometheus.CounterVec
	ClientConnectionCounter *prometheus.CounterVec
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

	prometheus.MustRegister(m.IncomingMsgsCounter)
	prometheus.MustRegister(m.ClientConnectionCounter)

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
