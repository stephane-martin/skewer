// +build linux,!nonsystemd

package journald

import (
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
)

// TODO: provide a way to link statically to libsystemd

var Supported = true

type Reader struct {
	journal      *sdjournal.Journal
	stopchan     chan struct{}
	shutdownchan chan struct{}
	wgroup       *sync.WaitGroup
	logger       log15.Logger
	stasher      base.Stasher
}

type Converter func(map[string]string) *model.FullMessage

func EntryToSyslog(entry map[string]string) (m *model.SyslogMessage) {
	m = model.CleanFactory()
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
	return func(m map[string]string) *model.FullMessage {
		dest := make(map[string]string)
		var k, k2, v, v2 string
		var err error
		for k, v = range m {
			k2, err = decoder.String(k)
			if err == nil {
				v2, err = decoder.String(v)
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

func NewReader(logger log15.Logger) (*Reader, error) {
	var err error
	r := &reader{logger: logger}
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
	r.wgroup = &sync.WaitGroup{}
	r.shutdownchan = make(chan struct{})
	return r, nil
}

func (r *Reader) wait() chan struct{} {
	events := make(chan struct{})
	r.wgroup.Add(1)

	go func() {
		defer r.wgroup.Done()
		var ev int

		for {
			select {
			case <-r.stopchan:
				close(events)
				return
			case <-r.shutdownchan:
				close(events)
				return
			default:
				ev = r.journal.Wait(time.Second)
				if ev == sdjournal.SD_JOURNAL_APPEND || ev == sdjournal.SD_JOURNAL_INVALIDATE {
					close(events)
					return
				} else if ev == -int(syscall.EBADF) {
					r.logger.Debug("journal.Wait returned EBADF") // r.journal was closed
					close(events)
					return
				} else if ev != 0 {
					r.logger.Debug("journal.Wait event", "code", ev)
				}
			}
		}
	}()
	return events
}

func (r *Reader) Start(confID utils.MyULID) {
	r.stopchan = make(chan struct{})
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	r.wgroup.Add(1)
	go func() {
		defer func() {
			r.entries.Dispose()
			r.wgroup.Done()
		}()

		var err error
		var nb uint64
		var entry *sdjournal.JournalEntry
		converter := makeMapConverter("utf8", confID)
		var m *model.FullMessage

		for {
			// get entries from journald
		LoopGetEntries:
			for {
				select {
				case <-r.stopchan:
					return
				default:
					nb, err = r.journal.Next()
					if err != nil {
						return
					}
					if nb == 0 {
						select {
						case <-r.shutdownchan:
							return
						default:
							break LoopGetEntries
						}
					}
					entry, err = r.journal.GetEntry()
					if err != nil {
						return
					}
					f, nf := r.stasher.Stash(converter(entry.Fields))
					if nf != nil {
						r.logger.Warn("Non-fatal error stashing journal message", "error", nf)
					} else if f != nil {
						r.logger.Error("Fatal error stashing journal message", "error", f)
					} else {
						base.IncomingMsgsCounter.WithLabelValues("journald", hostname, "", "").Inc()
					}

				}
			}

			// wait that journald has more entries
			events := r.wait()
			select {
			case <-events:
			case <-r.stopchan:
				return
			}
		}
	}()
}

func (r *Reader) Stop() {
	if r.stopchan != nil {
		close(r.stopchan)
		r.wgroup.Wait()
	}
}

func (r *Reader) Shutdown() {
	close(r.shutdownchan)
	r.wgroup.Wait()
	if r.stopchan != nil {
		close(r.stopchan)
	}
	go func() {
		r.journal.Close()
	}()
}
