package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/errwrap"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/auditlogs"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/store"
)

type AuditService struct {
	store     store.Store
	metrics   *metrics.Metrics
	logger    log15.Logger
	wgroup    *sync.WaitGroup
	generator chan ulid.ULID
}

func NewAuditService(st store.Store, generator chan ulid.ULID, metric *metrics.Metrics, logger log15.Logger) *AuditService {
	s := AuditService{store: st, metrics: metric, generator: generator}
	s.logger = logger.New("class", "audit")
	return &s
}

func (s *AuditService) Start(ctx context.Context, c conf.AuditConfig) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	msgChan, err := auditlogs.WriteAuditLogs(ctx, c)
	if err != nil {
		return err
	}

	syslogConf := &conf.SyslogConfig{
		FilterFunc:    c.FilterFunc,
		TopicFunc:     c.TopicFunc,
		TopicTmpl:     c.TopicTmpl,
		PartitionFunc: c.PartitionFunc,
		PartitionTmpl: c.PartitionTmpl,
	}

	confId := "fakeConfId"
	if s.store != nil {
		confId, err = s.store.StoreSyslogConfig(syslogConf)
		if err != nil {
			return errwrap.Wrapf("Error persisting the audit service configuration to the Store: {{err}}", err)
		}
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
			Appname:       c.Appname,
			Facility:      model.Facility(c.Facility),
			Severity:      model.Severity(c.Severity),
			Priority:      model.Priority(8*c.Facility + c.Severity),
			Hostname:      hostname,
			TimeReported:  treported,
			TimeGenerated: tgenerated,
			Msgid:         strconv.FormatInt(int64(auditMsg.Seq), 10),
			Procid:        "",
			AuditMessage:  auditMsg.Msgs,
		}

		if len(auditMsg.UidMap) > 0 {
			m.Properties = map[string]interface{}{}
			props := map[string]map[string]string{}
			props["uid_map"] = auditMsg.UidMap
			m.Properties["audit"] = props
		}

		return &m
	}

	var inputsChan chan *model.TcpUdpParsedMessage
	if s.store != nil {
		inputsChan = s.store.Inputs()
	}

	s.wgroup = &sync.WaitGroup{}
	s.wgroup.Add(1)
	go func() {
		for msg := range msgChan {
			uid := <-s.generator
			m := auditToSyslog(msg)
			parsed := &model.ParsedMessage{
				Client:         "audit",
				Fields:         m,
				LocalPort:      0,
				UnixSocketPath: "",
			}
			full := &model.TcpUdpParsedMessage{
				ConfId: confId,
				Uid:    uid.String(),
				Parsed: parsed,
			}
			if inputsChan != nil {
				inputsChan <- full
			} else {
				marsh, _ := json.Marshal(full)
				fmt.Println(string(marsh))
			}
		}
		s.wgroup.Done()
	}()
	return nil
}

func (s *AuditService) WaitFinished() {
	s.wgroup.Wait()
}
