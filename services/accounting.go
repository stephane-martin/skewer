package services

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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

func readFileUntilEnd(f *os.File, size int) (err error) {
	// read the acct file until the end
	buf := make([]byte, accounting.Ssize)
	for {
		_, err = io.ReadAtLeast(f, buf, size)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("Unexpected error while reading the accounting file: %s", err)
		}
	}
}

func (s *AccountingService) makeMessage(buf []byte, tick int64, hostname string) *model.FullMessage {
	acct := accounting.MakeAcct(buf, tick)
	uid := <-s.generator
	msg := model.FullMessage{
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
	}
	return &msg
}

func (s *AccountingService) readFile(f *os.File, tick int64, hostname string, size int) (err error) {
	buf := make([]byte, accounting.Ssize)
	for {
		_, err = io.ReadAtLeast(f, buf, size)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("Unexpected error while reading the accounting file: %s", err)
		} else {
			s.stasher.Stash(*(s.makeMessage(buf, tick, hostname)))
			s.metrics.IncomingMsgsCounter.WithLabelValues("accounting", hostname, "", "").Inc()
		}
	}
}

func (s *AccountingService) doStart(watcher *fsnotify.Watcher, hostname string, f *os.File, tick int64) {
	defer s.wgroup.Done()
	defer f.Close()
	var err error

	err = watcher.Add(s.Conf.Path)
	if err != nil {
		s.logger.Error("Error starting to watch accounting file")
		return
	}

	// fetch content from the acct file
	for {
		err := s.readFile(f, tick, hostname, accounting.Ssize)
		if err != nil {
			s.logger.Error(err.Error())
			watcher.Close()
			return
		}

	WaitWrite:
		for {
			select {
			case err := <-watcher.Errors:
				s.logger.Warn("Watcher error", "error", err)
			case ev := <-watcher.Events:
				switch ev.Op {
				case fsnotify.Write:
					break WaitWrite
				case fsnotify.Rename:
					// accounting file rotation
					s.logger.Info("Accounting file rotation")
					time.Sleep(3 * time.Second)
					f2, err := os.Open(s.Conf.Path)
					if err == nil {
						s.logger.Info("Accounting file has been reopened")
					} else {
						s.logger.Error("Error reopening accounting file", "error", err)
						return
					}
					s.wgroup.Add(1)
					go s.doStart(watcher, hostname, f2, tick)
					return
				case fsnotify.Remove:
					s.logger.Error("Accounting file has been removed ?!")
					watcher.Close()
					return
				default:
				}
			case <-s.stopchan:
				watcher.Close()
				return
			}
		}

	}

}

func (s *AccountingService) Start(test bool) (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	s.stopchan = make(chan struct{})
	tick := accounting.Tick()
	var f *os.File

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	f, err = os.Open(s.Conf.Path)
	if err != nil {
		return
	}

	err = readFileUntilEnd(f, accounting.Ssize)
	if err != nil {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	s.wgroup.Add(1)
	go s.doStart(watcher, hostname, f, tick)
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
