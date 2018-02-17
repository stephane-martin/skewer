package metrics

//go:generate goderive .

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stephane-martin/skewer/conf"
)

type MetricsServer struct {
	server *http.Server
}

func (m *MetricsServer) Stop() {
	if m.server != nil {
		_ = m.server.Close()
		m.server = nil
	}
}

type Logger struct {
	log15.Logger
}

func (l Logger) Println(v ...interface{}) {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintln(buf, v...)
	l.Debug(buf.String())
}

func (m *MetricsServer) NewConf(c conf.MetricsConfig, logger log15.Logger, gatherers ...prometheus.Gatherer) {
	m.Stop()
	var nonNilGatherers prometheus.Gatherers = filterGatherers(func(g prometheus.Gatherer) bool { return g != nil }, gatherers)
	logger.Debug("Number of metric gatherers", "nb", len(nonNilGatherers))

	if strings.TrimSpace(c.Path) == "" {
		c.Path = "/metrics"
	}
	if c.Port > 0 {
		mux := http.NewServeMux()
		mux.Handle(
			c.Path,
			promhttp.HandlerFor(
				nonNilGatherers,
				promhttp.HandlerOpts{
					ErrorLog:      Logger{Logger: logger},
					ErrorHandling: promhttp.HTTPErrorOnError,
				},
			),
		)
		m.server = &http.Server{
			Addr:    fmt.Sprintf("127.0.0.1:%d", c.Port),
			Handler: mux,
		}

		go func() {
			// actually listen
			err := m.server.ListenAndServe()
			if err != nil {
				if err == http.ErrServerClosed {
					logger.Info("Metrics HTTP server has been shut down")
				} else {
					logger.Error("Error starting the HTTP metric server", "error", err)
				}
			}
		}()
	}
}

func filterGatherers(predicate func(prometheus.Gatherer) bool, list []prometheus.Gatherer) []prometheus.Gatherer {
	j := 0
	for i, elem := range list {
		if predicate(elem) {
			if i != j {
				list[j] = list[i]
			}
			j++
		}
	}
	return list[:j]
}
