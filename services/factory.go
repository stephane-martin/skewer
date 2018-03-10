package services

import (
	"fmt"
	"os"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/services/linux"
	"github.com/stephane-martin/skewer/services/macos"
	"github.com/stephane-martin/skewer/services/network"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/kring"
)

func Configure(t base.Types, c conf.BaseConfig) (res conf.BaseConfig) {
	res = conf.NewBaseConf()
	res.Main.EncryptIPC = c.Main.EncryptIPC
	switch t {
	case base.TCP:
		res.TCPSource = c.TCPSource
		res.Parsers = c.Parsers
		res.Main.InputQueueSize = c.Main.InputQueueSize
		res.Main.MaxInputMessageSize = c.Main.MaxInputMessageSize
	case base.UDP:
		res.UDPSource = c.UDPSource
		res.Parsers = c.Parsers
		res.Main.InputQueueSize = c.Main.InputQueueSize
	case base.RELP:
		res.RELPSource = c.RELPSource
		res.Parsers = c.Parsers
		res.Main.InputQueueSize = c.Main.InputQueueSize
	case base.DirectRELP:
		res.DirectRELPSource = c.DirectRELPSource
		res.Parsers = c.Parsers
		res.Main.InputQueueSize = c.Main.InputQueueSize
		res.KafkaDest = c.KafkaDest
	case base.KafkaSource:
		res.KafkaSource = c.KafkaSource
		res.Parsers = c.Parsers
		res.Main.InputQueueSize = c.Main.InputQueueSize
	case base.Graylog:
		res.GraylogSource = c.GraylogSource
	case base.Journal:
		res.Journald = c.Journald
	case base.Accounting:
		res.Accounting = c.Accounting
	case base.Store:
		res = c
	case base.Filesystem:
		res.FSSource = c.FSSource
		res.Parsers = c.Parsers
	case base.HTTPServer:
		res.HTTPServerSource = c.HTTPServerSource
		res.Parsers = c.Parsers
		res.Main.InputQueueSize = c.Main.InputQueueSize
		res.Main.MaxInputMessageSize = c.Main.MaxInputMessageSize
	case base.MacOS:
		res.MacOS = c.MacOS
	}
	return res
}

func ConfigureAndStartService(s base.Provider, c conf.BaseConfig) ([]model.ListenerInfo, error) {
	switch s.Type() {
	case base.Store:
		return s.(*storeServiceImpl).SetConfAndRestart(c)
	default:
		s.SetConf(c)
		return s.Start()
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

type ProviderOpt func(e *base.ProviderEnv)

func ProviderFactory(t base.Types, env *base.ProviderEnv) (base.Provider, error) {
	var provider base.Provider
	var err error
	switch t {
	case base.TCP:
		provider, err = network.NewTcpService(env)
	case base.UDP:
		provider, err = network.NewUdpService(env)
	case base.RELP:
		provider, err = network.NewRelpService(env)
	case base.DirectRELP:
		provider, err = network.NewDirectRelpService(env)
	case base.Graylog:
		provider, err = network.NewGraylogService(env)
	case base.Journal:
		provider, err = linux.NewJournalService(env)
	case base.Accounting:
		provider, err = NewAccountingService(env)
	case base.Store:
		provider, err = NewStoreService(env)
	case base.KafkaSource:
		provider, err = network.NewKafkaService(env)
	case base.Filesystem:
		provider, err = NewFilePollingService(env)
	case base.HTTPServer:
		provider, err = network.NewHTTPService(env)
	case base.MacOS:
		provider, err = macos.NewMacOSLogsService(env)
	default:
		return nil, fmt.Errorf("unknown provider type: %d", t)
	}
	if err != nil {
		return nil, err
	}
	return provider, nil
}
