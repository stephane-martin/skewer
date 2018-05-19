// +build linux,!nonsystemd

package journald

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

var Supported = true

type Reader struct {
	journal        *sdjournal.Journal
	stop           context.CancelFunc
	wgroup         sync.WaitGroup
	logger         log15.Logger
	stasher        *base.Reporter
	fatalErrorChan chan struct{}
	fatalOnce      sync.Once
}

type Converter func(*sdjournal.JournalEntry) *model.FullMessage

func EntryToSyslog(entry map[string]string) *model.SyslogMessage {
	m := model.Factory()
	properties := map[string]string{}
	for k, v := range entry {
		k = strings.ToLower(k)
		switch k {
		case "syslog_identifier":
		case "_comm":
			m.AppName = v
		case "message":
			m.Message = v
		case "syslog_pid":
		case "_pid":
			m.ProcId = v
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
			m.HostName = v
		case "_source_realtime_timestamp": // microseconds
			t, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				m.TimeReportedNum = t * 1000
			}
		default:
			if strings.HasPrefix(k, "_") {
				properties[k] = v
			}

		}
	}
	if len(m.AppName) == 0 {
		m.AppName = entry["SYSLOG_IDENTIFIER"]
	}
	if len(m.ProcId) == 0 {
		m.ProcId = entry["SYSLOG_PID"]
	}
	m.TimeGeneratedNum = time.Now().UnixNano()
	if m.TimeReportedNum == 0 {
		m.TimeReportedNum = m.TimeGeneratedNum
	}
	m.Priority = model.Priority(int(m.Facility)*8 + int(m.Severity))
	m.ClearDomain("journald")
	m.Properties.Map["journald"].Map = properties
	m.SetProperty("skewer", "client", m.HostName)
	return m
}

func makeMapConverter(coding string, confID utils.MyULID) Converter {
	decoder := utils.SelectDecoder(coding)
	generator := utils.NewGenerator()

	return func(m *sdjournal.JournalEntry) *model.FullMessage {
		dest := make(map[string]string, len(m.Fields))
		for k, v := range m.Fields {
			k2, err := decoder.String(k)
			if err == nil {
				v2, err := decoder.String(v)
				if err == nil {
					dest[k2] = v2
				}
			}
		}
		full := model.FullFactoryFrom(EntryToSyslog(dest))
		full.Uid = generator.Uid()
		full.ConfId = confID
		return full
	}
}

func (r *Reader) FatalError() chan struct{} {
	return r.fatalErrorChan
}

func (r *Reader) dofatal() {
	r.fatalOnce.Do(func() { close(r.fatalErrorChan) })
}

func NewReader(stasher *base.Reporter, logger log15.Logger) (*Reader, error) {
	var err error
	r := &Reader{
		logger:         logger,
		stasher:        stasher,
		fatalErrorChan: make(chan struct{}),
	}
	r.journal, err = sdjournal.NewJournal()
	if err != nil {
		return nil, err
	}
	err = r.journal.SeekTail()
	if err != nil {
		r.journal.Close()
		return nil, err
	}
	_, err = r.journal.Previous()
	if err != nil {
		r.journal.Close()
		return nil, err
	}
	return r, nil
}

func wait(ctx context.Context, logger log15.Logger, j *sdjournal.Journal) {
	lctx, lcancel := context.WithCancel(ctx)

	go func() {
		for {
			// Wait() is a blocking call
			ev := j.Wait(time.Second)
			if ev == sdjournal.SD_JOURNAL_APPEND || ev == sdjournal.SD_JOURNAL_INVALIDATE {
				lcancel()
				return
			} else if ev == -int(syscall.EBADF) {
				logger.Debug("journal.Wait returned EBADF") // r.journal was closed
				lcancel()
				return
			} else if ev != 0 {
				// r.logger.Debug("journal.Wait event", "code", ev)
			}
		}
	}()
	<-lctx.Done()
}

func (r *Reader) Start(confID utils.MyULID) {
	var ctx context.Context
	ctx, r.stop = context.WithCancel(context.Background())
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	converter := makeMapConverter("utf8", confID)

	r.wgroup.Add(1)
	go func() {
		defer r.wgroup.Done()

	L:
		for {
			select {
			case <-ctx.Done():
				return
			default:
				nb, err := r.journal.Next()
				if err != nil {
					return
				}
				if nb == 0 {
					wait(ctx, r.logger, r.journal) // wait that journald has more entries
					continue L
				}
				entry, err := r.journal.GetEntry()
				if err != nil {
					return
				}
				err = r.stasher.Stash(converter(entry))
				if eerrors.IsFatal(err) {
					r.logger.Error("Fatal error stashing journal message", "error", err)
					r.dofatal()
					return
				}
				if err != nil {
					r.logger.Warn("Non-fatal error stashing journal message", "error", err)
					continue L
				}
				base.CountIncomingMessage(base.Journal, hostname, 0, "")
			}
		}

	}()
}

func (r *Reader) WaitFinished() {
	r.wgroup.Wait()
}

func (r *Reader) Stop() {
	if r.stop != nil {
		r.stop()
		r.stop = nil
	}
	r.WaitFinished()
}

func (r *Reader) Shutdown() {
	r.Stop()
	// async close the low level journald reader
	go func() {
		r.journal.Close()
	}()
}
