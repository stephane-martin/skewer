package linux

import (
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
)

type journalMetrics struct {
	IncomingMsgsCounter *prometheus.CounterVec
}

func NewJournalMetrics() *journalMetrics {
	m := &journalMetrics{}
	m.IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_incoming_messages_total",
			Help: "total number of messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)
	return m
}

type JournalService struct {
	stasher        *base.Reporter
	reader         journald.JournaldReader
	logger         log15.Logger
	Conf           conf.JournaldConfig
	wgroup         *sync.WaitGroup
	metrics        *journalMetrics
	registry       *prometheus.Registry
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
}

func NewJournalService(stasher *base.Reporter, l log15.Logger) (*JournalService, error) {
	s := JournalService{
		stasher:  stasher,
		metrics:  NewJournalMetrics(),
		registry: prometheus.NewRegistry(),
		logger:   l.New("class", "journald"),
		wgroup:   &sync.WaitGroup{},
	}
	s.registry.MustRegister(s.metrics.IncomingMsgsCounter)
	return &s, nil
}

func (s *JournalService) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *JournalService) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *JournalService) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *JournalService) Start(test bool) (infos []model.ListenerInfo, err error) {
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
				f, nf := s.stasher.Stash(*m)
				if nf != nil {
					s.logger.Warn("Non-fatal error stashing journal message", "error", nf)
				} else if f != nil {
					s.logger.Error("Fatal error stashing journal message", "error", f)
					s.dofatal()
				} else {
					s.metrics.IncomingMsgsCounter.WithLabelValues("journald", hostname, "", "").Inc()
				}
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

func (s *JournalService) SetConf(c conf.JournaldConfig) {
	s.Conf = c
}
