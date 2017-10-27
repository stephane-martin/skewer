package services

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/accounting"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
)

type accountingMetrics struct {
	IncomingMsgsCounter *prometheus.CounterVec
}

func NewAccountingMetrics() *accountingMetrics {
	m := &accountingMetrics{}
	m.IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_incoming_messages_total",
			Help: "total number of messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)
	return m
}

type AccountingService struct {
	stasher   *base.Reporter
	logger    log15.Logger
	wgroup    *sync.WaitGroup
	generator chan ulid.ULID
	metrics   *accountingMetrics
	registry  *prometheus.Registry
	Conf      conf.AccountingConfig
	stopchan  chan struct{}
}

func NewAccountingService(stasher *base.Reporter, gen chan ulid.ULID, l log15.Logger) (*AccountingService, error) {
	s := AccountingService{
		stasher:   stasher,
		logger:    l.New("class", "accounting"),
		wgroup:    &sync.WaitGroup{},
		generator: gen,
		metrics:   NewAccountingMetrics(),
		registry:  prometheus.NewRegistry(),
	}
	s.registry.MustRegister(s.metrics.IncomingMsgsCounter)
	return &s, nil
}

func (s *AccountingService) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *AccountingService) Start(test bool) (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	s.stopchan = make(chan struct{})
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	f, err := os.Open(s.Conf.Path)
	if err != nil {
		return
	}
	tick := accounting.Tick()

	s.wgroup.Add(1)
	go func() {
		defer func() {
			f.Close()
			s.wgroup.Done()
		}()
		buf := make([]byte, accounting.Ssize)
		var err error
		var acct accounting.Acct
		var uid ulid.ULID
		// read the acct file until the end
		for {
			_, err = io.ReadAtLeast(f, buf, accounting.Ssize)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
		}
		// fetch more content from the acct file
		for {
			_, err = io.ReadAtLeast(f, buf, accounting.Ssize)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				select {
				case <-time.After(s.Conf.Period):
				case <-s.stopchan:
					return
				}
			} else if err != nil {
				s.logger.Error("Unexpected error while reading the accounting file", "error", err)
				return
			} else {
				acct = accounting.MakeAcct(buf, tick)
				uid = <-s.generator
				s.stasher.Stash(model.FullMessage{
					ConfId: s.Conf.ConfID,
					Uid:    uid,
					Parsed: model.ParsedMessage{
						Client:         hostname,
						LocalPort:      0,
						UnixSocketPath: "",
						Fields: model.SyslogMessage{
							Appname:          "accounting",
							Facility:         0,
							Hostname:         hostname,
							Msgid:            "",
							Priority:         0,
							Procid:           "",
							Severity:         0,
							Properties:       map[string]map[string]string{"acct": acct.Properties()},
							Structured:       "",
							TimeGeneratedNum: acct.Btime.UnixNano(),
							TimeReportedNum:  time.Now().UnixNano(),
							Version:          0,
							Message:          "Process accounting message",
						},
					},
				})
				s.metrics.IncomingMsgsCounter.WithLabelValues("accounting", hostname, "", "").Inc()
			}
		}

	}()
	return
}

func (s *AccountingService) Stop() {
	if s.stopchan != nil {
		close(s.stopchan)
		s.stopchan = nil
	}
	s.wgroup.Wait()
}

func (s *AccountingService) Shutdown() {
	if s.stopchan != nil {
		close(s.stopchan)
		s.stopchan = nil
	}
	s.wgroup.Wait()
}

func (s *AccountingService) SetConf(c conf.AccountingConfig) {
	s.Conf = c
}
