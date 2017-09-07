package linux

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/model"
)

func EntryToSyslog(entry map[string]string) *model.SyslogMessage {
	m := model.SyslogMessage{}
	properties := map[string]string{}
	for k, v := range entry {
		k = strings.ToLower(k)
		switch k {
		case "syslog_identifier":
		case "_comm":
			m.Appname = v
		case "message":
			m.Message = v
		case "syslog_pid":
		case "_pid":
			m.Procid = v
		case "priority":
			p, err := strconv.Atoi(v)
			if err == nil {
				m.Severity = model.Severity(p)
			}
		case "syslog_facility":
			f, err := strconv.Atoi(v)
			if err == nil {
				m.Facility = model.Facility(f)
			}
		case "_hostname":
			m.Hostname = v
		case "_source_realtime_timestamp": // microseconds
			t, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				m.TimeReported = time.Unix(0, t*1000)
			}
		default:
			if strings.HasPrefix(k, "_") {
				properties[k] = v
			}

		}
	}
	if len(m.Appname) == 0 {
		m.Appname = entry["SYSLOG_IDENTIFIER"]
	}
	if len(m.Procid) == 0 {
		m.Procid = entry["SYSLOG_PID"]
	}
	m.TimeGenerated = time.Now()
	if m.TimeReported.IsZero() {
		m.TimeReported = m.TimeGenerated
	}
	m.Priority = model.Priority(int(m.Facility)*8 + int(m.Severity))
	m.Properties = map[string]map[string]string{}
	m.Properties["journald"] = properties
	return &m
}

type journalMetrics struct {
	IncomingMsgsCounter *prometheus.CounterVec
}

func NewJournalMetrics() *journalMetrics {
	m := &journalMetrics{}
	m.IncomingMsgsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "skw_incoming_messages_total",
			Help: "total number of syslog messages that were received",
		},
		[]string{"protocol", "client", "port", "path"},
	)
	return m
}

type JournalService struct {
	stasher   model.Stasher
	reader    journald.JournaldReader
	logger    log15.Logger
	stopchan  chan struct{}
	Conf      conf.JournaldConfig
	wgroup    *sync.WaitGroup
	generator chan ulid.ULID
	metrics   *journalMetrics
	registry  *prometheus.Registry
}

func NewJournalService(stasher model.Stasher, gen chan ulid.ULID, l log15.Logger) (*JournalService, error) {
	var err error
	s := JournalService{
		stasher:   stasher,
		generator: gen,
		metrics:   NewJournalMetrics(),
		registry:  prometheus.NewRegistry(),
		logger:    l.New("class", "journald"),
		wgroup:    &sync.WaitGroup{},
	}
	s.registry.MustRegister(s.metrics.IncomingMsgsCounter)
	s.reader, err = journald.NewReader(s.logger)
	if err != nil {
		return nil, err
	}
	s.reader.Start()
	return &s, nil
}

func (s *JournalService) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *JournalService) Start(test bool) ([]*model.ListenerInfo, error) {
	s.wgroup.Add(1)
	s.stopchan = make(chan struct{})
	go func() {
		defer s.wgroup.Done()

		for {
			select {
			case entry, more := <-s.reader.Entries():
				if more {
					message := EntryToSyslog(entry)
					uid := <-s.generator
					parsedMessage := model.ParsedMessage{
						Client:         "journald",
						LocalPort:      0,
						UnixSocketPath: "",
						Fields:         message,
					}
					fullParsedMessage := model.TcpUdpParsedMessage{
						ConfId: s.Conf.ConfID,
						Uid:    uid.String(),
						Parsed: &parsedMessage,
					}
					if s.stasher != nil {
						s.stasher.Stash(&fullParsedMessage)
					}
					if s.metrics != nil {
						s.metrics.IncomingMsgsCounter.WithLabelValues("journald", "journald", "", "").Inc()
					}
				} else {
					return
				}
			case <-s.stopchan:
				return
			}
		}
	}()
	s.logger.Debug("Journald service is started")
	return []*model.ListenerInfo{}, nil
}

func (s *JournalService) Stop() {
	close(s.stopchan)
	s.wgroup.Wait()
}

func (s *JournalService) Shutdown() {
	s.Stop()
	s.reader.Shutdown()
}

func (s *JournalService) WaitClosed() {
	s.wgroup.Wait()
}

func (s *JournalService) SetConf(c conf.JournaldConfig) {
	s.Conf = c
}
