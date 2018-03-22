package store

import (
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

type Store interface {
	Stash(uid utils.MyULID, b []byte) (error, error)
	Outputs(dest conf.DestinationType) chan []*model.FullMessage
	ACK(uid utils.MyULID, dest conf.DestinationType)
	NACK(uid utils.MyULID, dest conf.DestinationType)
	PermError(uid utils.MyULID, dest conf.DestinationType)
	Errors() chan struct{}
	WaitFinished()
	GetSyslogConfig(configID utils.MyULID) (*conf.FilterSubConfig, error)
	StoreAllSyslogConfigs(c conf.BaseConfig) error
	ReadAllBadgers() (map[string]string, map[string]string, map[string]string)
	Destinations() []conf.DestinationType
	Confined() bool
}
