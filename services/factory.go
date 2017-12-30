package services

import (
	"fmt"
	"os"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/services/linux"
	"github.com/stephane-martin/skewer/services/network"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/kring"
)

type Types int

const (
	TCP Types = iota
	UDP
	RELP
	DirectRELP
	Journal
	Store
	Accounting
	KafkaSource
	Configuration
	Graylog
)

var Names2Types map[string]Types = map[string]Types{
	"skewer-tcp":         TCP,
	"skewer-udp":         UDP,
	"skewer-relp":        RELP,
	"skewer-directrelp":  DirectRELP,
	"skewer-journal":     Journal,
	"skewer-store":       Store,
	"skewer-accounting":  Accounting,
	"skewer-kafkasource": KafkaSource,
	"skewer-conf":        Configuration,
	"skewer-graylog":     Graylog,
}

var Types2Names map[Types]string
var Types2ConfinedNames map[Types]string

var Handles []ServiceHandle
var HandlesMap map[ServiceHandle]uintptr

type HandleType uint8

const (
	Binder HandleType = iota
	Logger
)

type ServiceHandle struct {
	Service string
	Type    HandleType
}

func init() {
	Types2Names = map[Types]string{}
	Types2ConfinedNames = map[Types]string{}
	for k, v := range Names2Types {
		Types2Names[v] = k
		Types2ConfinedNames[v] = "confined-" + k
	}

	Handles = []ServiceHandle{
		{"child", Binder},
		{Types2Names[TCP], Binder},
		{Types2Names[UDP], Binder},
		{Types2Names[RELP], Binder},
		{Types2Names[DirectRELP], Binder},
		{Types2Names[Store], Binder},
		{Types2Names[Graylog], Binder},
		{"child", Logger},
		{Types2Names[TCP], Logger},
		{Types2Names[UDP], Logger},
		{Types2Names[RELP], Logger},
		{Types2Names[DirectRELP], Logger},
		{Types2Names[Journal], Logger},
		{Types2Names[Configuration], Logger},
		{Types2Names[Store], Logger},
		{Types2Names[Accounting], Logger},
		{Types2Names[KafkaSource], Logger},
		{Types2Names[Graylog], Logger},
	}

	HandlesMap = map[ServiceHandle]uintptr{}
	for i, h := range Handles {
		HandlesMap[h] = uintptr(i + 3)
	}
}

func LoggerHdl(typ Types) uintptr {
	return HandlesMap[ServiceHandle{Types2Names[typ], Logger}]
}

func BinderHdl(typ Types) uintptr {
	return HandlesMap[ServiceHandle{Types2Names[typ], Binder}]
}

// TODO: refactor
func ConfigureAndStartService(s base.Provider, c conf.BaseConfig) ([]model.ListenerInfo, error) {

	switch s := s.(type) {
	case *network.TcpServiceImpl:
		s.SetConf(c.TCPSource, c.Parsers, c.Main.InputQueueSize, c.Main.MaxInputMessageSize)
		return s.Start()
	case *network.UdpServiceImpl:
		s.SetConf(c.UDPSource, c.Parsers, c.Main.InputQueueSize)
		return s.Start()
	case *network.RelpService:
		s.SetConf(c.RELPSource, c.Parsers, c.Main.InputQueueSize)
		return s.Start()
	case *network.DirectRelpService:
		s.SetConf(c.DirectRELPSource, c.Parsers, c.KafkaDest, c.Main.InputQueueSize)
		return s.Start()
	case *network.GraylogSvcImpl:
		s.SetConf(c.GraylogSource)
		return s.Start()
	case *linux.JournalService:
		s.SetConf(c.Journald)
		return s.Start()
	case *AccountingService:
		s.SetConf(c.Accounting)
		return s.Start()
	case *storeServiceImpl:
		return s.SetConfAndRestart(c)
	case *network.KafkaServiceImpl:
		s.SetConf(c.KafkaSource, c.Parsers, c.Main.InputQueueSize)
		return s.Start()
	default:
		return nil, fmt.Errorf("Unknown network service: %T", s)
	}

}

func SetConfined(confined bool) func(e *base.ProviderEnv) {
	return func(e *base.ProviderEnv) {
		e.Confined = confined
	}
}

func SetProfile(profile bool) func(e *base.ProviderEnv) {
	return func(e *base.ProviderEnv) {
		e.Profile = profile
	}
}

func SetRing(ring kring.Ring) func(e *base.ProviderEnv) {
	return func(e *base.ProviderEnv) {
		e.Ring = ring
	}
}

func SetReporter(reporter base.Reporter) func(e *base.ProviderEnv) {
	return func(e *base.ProviderEnv) {
		e.Reporter = reporter
	}
}

func SetLogger(logger log15.Logger) func(e *base.ProviderEnv) {
	return func(e *base.ProviderEnv) {
		e.Logger = logger
	}
}

func SetBinder(bindr binder.Client) func(e *base.ProviderEnv) {
	return func(e *base.ProviderEnv) {
		e.Binder = bindr
	}
}

func SetPipe(pipe *os.File) func(e *base.ProviderEnv) {
	return func(e *base.ProviderEnv) {
		e.Pipe = pipe
	}
}

type ProviderConstructor func(*base.ProviderEnv) (base.Provider, error)

var constructors map[Types]ProviderConstructor = map[Types]ProviderConstructor{
	TCP:         network.NewTcpService,
	UDP:         network.NewUdpService,
	RELP:        network.NewRelpService,
	DirectRELP:  network.NewDirectRelpService,
	Graylog:     network.NewGraylogService,
	Journal:     linux.NewJournalService,
	Accounting:  NewAccountingService,
	Store:       NewStoreService,
	KafkaSource: network.NewKafkaService,
}

type ProviderOpt func(e *base.ProviderEnv)

func ProviderFactory(t Types, env *base.ProviderEnv) (base.Provider, error) {
	if constructor, ok := constructors[t]; ok {
		provider, err := constructor(env)
		if err == nil {
			return provider, nil
		}
		return nil, err
	}
	return nil, fmt.Errorf("Unknown provider type: %d", t)
}
