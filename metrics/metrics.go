package metrics

//go:generate goderive .

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stephane-martin/skewer/conf"
)

type MetricsServer struct {
	server *http.Server
}

func (m *MetricsServer) Stop() {
	if m.server != nil {
		m.server.Close()
		m.server = nil
	}
}

func (m *MetricsServer) NewConf(c conf.MetricsConfig, gatherers ...prometheus.Gatherer) {
	m.Stop()
	var nonNilGatherers prometheus.Gatherers = deriveFilterGatherers(func(g prometheus.Gatherer) bool { return g != nil }, gatherers)

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
