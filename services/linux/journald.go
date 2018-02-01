package linux

import (
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
)

func initJournalRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type JournalService struct {
	stasher        base.Stasher
	reader         journald.JournaldReader
	logger         log15.Logger
	Conf           conf.JournaldConfig
	wgroup         *sync.WaitGroup
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
}

func NewJournalService(env *base.ProviderEnv) (base.Provider, error) {
	initJournalRegistry()
	s := JournalService{
		stasher: env.Reporter,
		logger:  env.Logger.New("class", "journald"),
		wgroup:  &sync.WaitGroup{},
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
	return s.fatalErrorChan
}

func (s *JournalService) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *JournalService) Start() (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	if s.reader == nil {
		// create the low level journald reader if needed
		s.reader, err = journald.NewReader(s.logger)
		if err != nil {
			return infos, err
		}
	}
	s.reader.Start()
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}

	s.wgroup.Add(1)
	go func() {
		defer s.wgroup.Done()

		var m *model.FullMessage
		var err error
		q := s.reader.Entries()

		for q.Wait(0) {
			m, err = q.Get()
			if m != nil && err == nil {
				m.ConfId = s.Conf.ConfID
				f, nf := s.stasher.Stash(m)
				if nf != nil {
					s.logger.Warn("Non-fatal error stashing journal message", "error", nf)
				} else if f != nil {
					s.logger.Error("Fatal error stashing journal message", "error", f)
					s.dofatal()
				} else {
					base.IncomingMsgsCounter.WithLabelValues("journald", hostname, "", "").Inc()
				}
				model.FullFree(m)
			}
		}
	}()

	s.logger.Debug("Journald service has started")
	return infos, nil
}

func (s *JournalService) Stop() {
	s.reader.Stop() // ask the low-level journal reader to stop sending events to Entries()
	s.wgroup.Wait()
}

func (s *JournalService) Shutdown() {
	s.reader.Shutdown()
	s.wgroup.Wait()
}

func (s *JournalService) SetConf(c conf.BaseConfig) {
	s.Conf = c.Journald
}
