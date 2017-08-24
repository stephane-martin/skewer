package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/auditlogs"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type AuditService struct {
	stasher   model.Stasher
	logger    log15.Logger
	wgroup    *sync.WaitGroup
	generator chan ulid.ULID
	cancel    context.CancelFunc
	aconf     *conf.AuditConfig
}

func NewAuditService(stasher model.Stasher, generator chan ulid.ULID, logger log15.Logger) *AuditService {
	s := AuditService{stasher: stasher, generator: generator}
	s.logger = logger.New("class", "audit")
	return &s
}

func (s *AuditService) Gather() ([]*dto.MetricFamily, error) {
	return []*dto.MetricFamily{}, nil
}

func (s *AuditService) Start(test bool) ([]*model.ListenerInfo, error) {
	infos := []*model.ListenerInfo{}
	hostname, err := os.Hostname()
	if err != nil {
		return infos, err
	}

	var ctx context.Context
	ctx, s.cancel = context.WithCancel(context.Background())

	msgChan, err := auditlogs.WriteAuditLogs(ctx, s.aconf, s.logger)
	if err != nil {
		return infos, err
	}

	auditToSyslog := func(auditMsg *model.AuditMessageGroup) *model.SyslogMessage {
		tgenerated := time.Now()
		treported := tgenerated
		nbsecs, err := strconv.ParseFloat(auditMsg.AuditTime, 64)
		if err == nil {
			millisecs := int64(nbsecs * 1000)
			treported = time.Unix(0, millisecs*1000000)
		}

		m := model.SyslogMessage{
			Appname:          s.aconf.Appname,
			Facility:         model.Facility(s.aconf.Facility),
			Severity:         model.Severity(s.aconf.Severity),
			Priority:         model.Priority(8*s.aconf.Facility + s.aconf.Severity),
			Hostname:         hostname,
			TimeReported:     treported,
			TimeGenerated:    tgenerated,
			Msgid:            strconv.FormatInt(int64(auditMsg.Seq), 10),
			Procid:           "",
			AuditSubMessages: auditMsg.Msgs,
		}

		if len(auditMsg.UidMap) > 0 {
			m.Properties = map[string]map[string]string{}
			m.Properties["uid_map"] = auditMsg.UidMap
		}

		return &m
	}

	s.wgroup = &sync.WaitGroup{}
	s.wgroup.Add(1)
	go func() {
		for msg := range msgChan {
			uid := <-s.generator
			m := auditToSyslog(msg)
			parsed := &model.ParsedMessage{
				Client: "audit",
				Fields: m,
			}
			full := &model.TcpUdpParsedMessage{
				ConfId: s.aconf.ConfID,
				Uid:    uid.String(),
				Parsed: parsed,
			}
			if s.stasher != nil {
				s.stasher.Stash(full)
			} else {
				marsh, _ := json.Marshal(full)
				fmt.Println(string(marsh))
			}
		}
		s.wgroup.Done()
	}()
	return infos, nil
}

func (s *AuditService) SetConf(sc []*conf.SyslogConfig, pc []conf.ParserConfig) {}

func (s *AuditService) SetKafkaConf(kc *conf.KafkaConfig) {}

func (s *AuditService) SetAuditConf(ac *conf.AuditConfig) {
	s.aconf = ac
}

func (s *AuditService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *AuditService) WaitClosed() {
	s.wgroup.Wait()
}
