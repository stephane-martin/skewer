package services

import (
	"fmt"
	"os"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/linux"
	"github.com/stephane-martin/skewer/services/network"
	"github.com/stephane-martin/skewer/sys"
)

type NetworkServiceType int

const (
	TCP NetworkServiceType = iota
	UDP
	RELP
	Journal
	Audit
	Store
)

var NetworkServiceMap map[string]NetworkServiceType = map[string]NetworkServiceType{
	"skewer-tcp":     TCP,
	"skewer-udp":     UDP,
	"skewer-relp":    RELP,
	"skewer-journal": Journal,
	"skewer-audit":   Audit,
	"skewer-store":   Store,
}

var ReverseNetworkServiceMap map[NetworkServiceType]string

func init() {
	ReverseNetworkServiceMap = map[NetworkServiceType]string{}
	for k, v := range NetworkServiceMap {
		ReverseNetworkServiceMap[v] = k
	}
}

func ConfigureAndStartService(s NetworkService, c conf.BaseConfig, test bool) ([]model.ListenerInfo, error) {
	switch s := s.(type) {
	case *network.TcpServiceImpl:
		s.SetConf(c.Syslog, c.Parsers)
		return s.Start(test)
	case *network.UdpServiceImpl:
		s.SetConf(c.Syslog, c.Parsers)
		return s.Start(test)
	case *network.RelpService:
		s.SetConf(c.Syslog, c.Parsers, c.Kafka)
		return s.Start(test)
	case *linux.JournalService:
		s.SetConf(c.Journald)
		return s.Start(test)
	case *linux.AuditService:
		s.SetAuditConf(c.Audit)
		return s.Start(test)
	case *storeServiceImpl:
		return s.SetConfAndRestart(c, test)
	default:
		return nil, fmt.Errorf("Unknown network service: %T", s)
	}

}

func Factory(t NetworkServiceType, stasher model.Stasher, gen chan ulid.ULID, b *sys.BinderClient, l log15.Logger) NetworkService {
	switch t {
	case TCP:
		return network.NewTcpService(stasher, gen, b, l)
	case UDP:
		return network.NewUdpService(stasher, gen, b, l)
	case RELP:
		return network.NewRelpService(b, l)
	case Journal:
		svc, err := linux.NewJournalService(stasher, gen, l)
		if err == nil {
			return svc
		} else {
			l.Error("Error creating the journal service", "error", err)
			return nil
		}
	case Audit:
		return linux.NewAuditService(stasher, gen, l)
	case Store:
		return NewStoreService(l)
	default:
		fmt.Fprintf(os.Stderr, "Unknown service type: %d\n", t)
		return nil
	}
}
