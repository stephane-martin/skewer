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
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys"
)

func EntryToSyslog(entry map[string]string) model.ParsedMessage {
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
				m.TimeReported = t * 1000
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
	m.TimeGenerated = time.Now().UnixNano()
	if m.TimeReported == 0 {
		m.TimeReported = m.TimeGenerated
	}
	m.Priority = model.Priority(int(m.Facility)*8 + int(m.Severity))
	m.Properties = map[string]map[string]string{}
	m.Properties["journald"] = properties

	return model.ParsedMessage{
		Client:         "journald",
		LocalPort:      0,
		UnixSocketPath: "",
		Fields:         m,
	}
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
	stasher   *base.Reporter
	reader    journald.JournaldReader
	logger    log15.Logger
	stopchan  chan struct{}
	Conf      conf.JournaldConfig
	wgroup    *sync.WaitGroup
	generator chan ulid.ULID
	metrics   *journalMetrics
	registry  *prometheus.Registry
}

func NewJournalService(stasher *base.Reporter, gen chan ulid.ULID, l log15.Logger) (*JournalService, error) {
	s := JournalService{
		stasher:   stasher,
		generator: gen,
		metrics:   NewJournalMetrics(),
		registry:  prometheus.NewRegistry(),
		logger:    l.New("class", "journald"),
		wgroup:    &sync.WaitGroup{},
	}
	s.registry.MustRegister(s.metrics.IncomingMsgsCounter)
	if sys.CapabilitiesSupported {
		l.Debug("Capabilities", "caps", sys.GetCaps())
	}
	return &s, nil
}

func (s *JournalService) Gather() ([]*dto.MetricFamily, error) {
	return s.registry.Gather()
}

func (s *JournalService) Start(test bool) (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	if s.reader == nil {
		// start the journald reader if needed
		s.reader, err = journald.NewReader(s.logger)
		if err != nil {
			return infos, err
		}
		s.reader.Start(s.Conf.Encoding)
	}
	s.wgroup.Add(1)
	s.stopchan = make(chan struct{})

	go func() {
		defer s.wgroup.Done()

		var uid ulid.ULID
		var entry map[string]string
		var more bool

		for {
			select {
			case entry, more = <-s.reader.Entries():
				if more {
					uid = <-s.generator
					s.stasher.Stash(model.TcpUdpParsedMessage{
						ConfId: s.Conf.ConfID,
						Uid:    uid.String(),
						Parsed: EntryToSyslog(entry),
					})
					s.metrics.IncomingMsgsCounter.WithLabelValues("journald", "journald", "", "").Inc()
				} else {
					return
				}
			case <-s.stopchan:
				return
			}
		}
	}()
	s.logger.Debug("Journald service has started")
	return infos, nil
}

func (s *JournalService) Stop() {
	close(s.stopchan)
	s.wgroup.Wait()
}

func (s *JournalService) Shutdown() {
	s.Stop()
	s.reader.Shutdown()
}

func (s *JournalService) SetConf(c conf.JournaldConfig) {
	s.Conf = c
}
