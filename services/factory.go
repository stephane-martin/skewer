package services

import (
	"context"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/linux"
	"github.com/stephane-martin/skewer/services/network"
	"github.com/stephane-martin/skewer/sys"
)

type NetworkService interface {
	Start(test bool) ([]*model.ListenerInfo, error)
	Stop()
	WaitClosed()
	SetConf(sc []*conf.SyslogConfig, pc []conf.ParserConfig)
	SetKafkaConf(kc *conf.KafkaConfig)
	SetAuditConf(ac *conf.AuditConfig)
	Gather() ([]*dto.MetricFamily, error)
}

func Factory(t string, stasher model.Stasher, gen chan ulid.ULID, b *sys.BinderClient, l log15.Logger) (NetworkService, context.CancelFunc) {
	switch t {
	case "skewer-tcp":
		return network.NewTcpService(stasher, gen, b, l), nil
	case "skewer-udp":
		return network.NewUdpService(stasher, gen, b, l), nil
	case "skewer-relp":
		return network.NewRelpService(b, l), nil
	case "skewer-journal":
		ctx, cancel := context.WithCancel(context.Background())
		s, err := linux.NewJournalService(ctx, stasher, gen, l)
		if err == nil {
			return s, cancel
		} else {
			l.Error("Error initializing journal service", "error", err)
			cancel()
			return nil, nil
		}
	case "skewer-audit":
		return linux.NewAuditService(stasher, gen, l), nil
	default:
		return nil, nil
	}
}
