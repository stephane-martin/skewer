package store

import (
	"context"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type Store interface {
	Stash(m *model.TcpUdpParsedMessage)
	Outputs() chan *model.TcpUdpParsedMessage
	ACK(uid string)
	NACK(uid string)
	PermError(uid string)
	Errors() chan struct{}
	WaitFinished()
	GetSyslogConfig(configID string) (*conf.SyslogConfig, error)
	//StoreSyslogConfig(config *conf.SyslogConfig) error
	StoreAllSyslogConfigs(c *conf.BaseConfig) error
	ReadAllBadgers() (map[string]string, map[string]string, map[string]string)
}

type Forwarder interface {
	Forward(ctx context.Context, from Store, to conf.KafkaConfig) bool
	ErrorChan() chan struct{}
	WaitFinished()
}
