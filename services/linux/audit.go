package linux

import (
	"context"
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
	"github.com/stephane-martin/skewer/services/base"
)

type AuditService struct {
	stasher   *base.Reporter
	logger    log15.Logger
	wgroup    *sync.WaitGroup
	generator chan ulid.ULID
	cancel    context.CancelFunc
	aconf     conf.AuditConfig
}

func NewAuditService(stasher *base.Reporter, generator chan ulid.ULID, logger log15.Logger) *AuditService {
	s := AuditService{stasher: stasher, generator: generator}
	s.logger = logger.New("class", "audit")
	return &s
}

func (s *AuditService) Gather() ([]*dto.MetricFamily, error) {
	return []*dto.MetricFamily{}, nil
}

func auditToSyslog(auditMsg *model.AuditMessageGroup, hostname string, aconf *conf.AuditConfig) model.ParsedMessage {
	tgenerated := time.Now().UnixNano()
	treported := tgenerated
	nbsecs, err := strconv.ParseFloat(auditMsg.AuditTime, 64)
	if err == nil {
		treported = int64(nbsecs*1000) * 1000000
	}

	m := model.SyslogMessage{
		Appname:          aconf.Appname,
		Facility:         model.Facility(aconf.Facility),
		Severity:         model.Severity(aconf.Severity),
		Priority:         model.Priority(8*aconf.Facility + aconf.Severity),
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

	return model.ParsedMessage{
		Client: "audit",
		Fields: m,
	}

}

func (s *AuditService) Start(test bool) ([]model.ListenerInfo, error) {
	infos := []model.ListenerInfo{}
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

	s.wgroup = &sync.WaitGroup{}
	s.wgroup.Add(1)
	go func() {
		defer s.wgroup.Done()

		var uid ulid.ULID
		var full model.TcpUdpParsedMessage

		for msg := range msgChan {
			uid = <-s.generator

			full = model.TcpUdpParsedMessage{
				ConfId: s.aconf.ConfID,
				Uid:    uid.String(),
				Parsed: auditToSyslog(msg, hostname, &s.aconf),
			}
			if s.stasher != nil {
				s.stasher.Stash(full)
			}
		}
	}()
	return infos, nil
}

func (s *AuditService) SetAuditConf(ac conf.AuditConfig) {
	s.aconf = ac
}

func (s *AuditService) Shutdown() {
	s.Stop()
}

func (s *AuditService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wgroup.Wait()
}
