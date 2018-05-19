package linux

import (
	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

func initJournalRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type JournalService struct {
	stasher *base.Reporter
	reader  journald.JournaldReader
	logger  log15.Logger
	Conf    conf.JournaldConfig
}

func NewJournalService(env *base.ProviderEnv) (base.Provider, error) {
	initJournalRegistry()
	s := JournalService{
		stasher: env.Reporter,
		logger:  env.Logger.New("class", "journald"),
	}
	var err error
	s.reader, err = journald.NewReader(s.stasher, s.logger)
	if err != nil {
		return nil, eerrors.Wrap(err, "Error creating the journald reader")
	}
	return &s, nil
}

func (s *JournalService) Type() base.Types {
	return base.Journal
}

func (s *JournalService) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *JournalService) FatalError() chan struct{} {
	return s.reader.FatalError()
}

func (s *JournalService) Start() (infos []model.ListenerInfo, err error) {
	infos = make([]model.ListenerInfo, 0)
	s.reader.Start(s.Conf.ConfID)
	s.logger.Debug("Journald service has started")
	return infos, nil
}

func (s *JournalService) Stop() {
	s.reader.Stop() // ask the low-level journal reader to stop sending events to Entries()
}

func (s *JournalService) Shutdown() {
	s.reader.Shutdown()
}

func (s *JournalService) SetConf(c conf.BaseConfig) {
	s.Conf = c.Journald
}
