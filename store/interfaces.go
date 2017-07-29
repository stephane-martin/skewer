package store

import (
	"context"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type Store interface {
	Inputs() chan *model.TcpUdpParsedMessage
	Outputs() chan *model.TcpUdpParsedMessage
	Ack() chan string
	Nack() chan string
	ProcessingErrors() chan string
	Errors() chan struct{}
	WaitFinished()
	GetSyslogConfig(configID string) (*conf.SyslogConfig, error)
	StoreSyslogConfig(config *conf.SyslogConfig) (string, error)
	ReadAllBadgers() (map[string]string, map[string]string, map[string]string)
}

type Forwarder interface {
	Forward(ctx context.Context, from Store, to conf.KafkaConfig) bool
	ErrorChan() chan struct{}
	WaitFinished()
}
