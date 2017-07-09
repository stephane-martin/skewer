package server

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/journald"
	"github.com/stephane-martin/relp2kafka/metrics"
	"github.com/stephane-martin/relp2kafka/model"
	"github.com/stephane-martin/relp2kafka/store"
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
	m.Properties = map[string]interface{}{}
	m.Properties["journald"] = properties
	return &m
}

type JournaldServer struct {
	store     store.Store
	reader    journald.JournaldReader
	metrics   *metrics.Metrics
	logger    log15.Logger
	stopchan  chan bool
	Conf      conf.JournaldConfig
	wgroup    *sync.WaitGroup
	generator chan ulid.ULID
}

func NewJournaldServer(
	ctx context.Context, c conf.JournaldConfig,
	st store.Store, generator chan ulid.ULID, metric *metrics.Metrics,
	logger log15.Logger,
) (*JournaldServer, error) {

	var err error
	s := JournaldServer{Conf: c, store: st, metrics: metric, generator: generator}
	s.logger = logger.New("class", "journald")
	s.reader, err = journald.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	s.wgroup = &sync.WaitGroup{}
	s.reader.Start()
	return &s, nil
}

func (s *JournaldServer) Start() {
	s.stopchan = make(chan bool)
	c := conf.SyslogConfig{
		FilterFunc:    s.Conf.FilterFunc,
		TopicFunc:     s.Conf.TopicFunc,
		TopicTmpl:     s.Conf.TopicTmpl,
		PartitionFunc: s.Conf.PartitionFunc,
		PartitionTmpl: s.Conf.PartitionTmpl,
	}
	confId, err := s.store.StoreSyslogConfig(&c)
	if err != nil {
		// ???
		return
	}
	inputs := s.store.Inputs()
	s.wgroup.Add(1)
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
						ConfId: confId,
						Uid:    uid.String(),
						Parsed: parsedMessage,
					}
					inputs <- &fullParsedMessage
				} else {
					return
				}
			case <-s.stopchan:
				return
			}
		}
	}()
}

func (s *JournaldServer) Stop() {
	if s.stopchan != nil {
		close(s.stopchan)
	}
	s.wgroup.Wait()
}
