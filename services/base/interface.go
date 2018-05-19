package base

import (
	"strconv"

	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type Provider interface {
	Start() ([]model.ListenerInfo, error)
	Stop()
	Shutdown()
	Gather() ([]*dto.MetricFamily, error)
	FatalError() chan struct{}
	Type() Types
	SetConf(c conf.BaseConfig)
}

func CountIncomingMessage(t Types, client string, port int, path string) {
	IncomingMsgsCounter.WithLabelValues(Types2Names[t], client, strconv.FormatInt(int64(port), 10), path).Inc()
}

func CountClientConnection(t Types, client string, port int, path string) {
	ClientConnectionCounter.WithLabelValues(Types2Names[t], client, strconv.FormatInt(int64(port), 10), path).Inc()
}

func CountParsingError(t Types, client string, parserName string) {
	ParsingErrorCounter.WithLabelValues(Types2Names[t], client, parserName).Inc()
}
