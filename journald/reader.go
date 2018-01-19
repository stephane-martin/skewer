// +build linux,!nonsystemd

package journald

import (
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
)

// TODO: provide a way to link statically to libsystemd

var Supported bool = true

type reader struct {
	journal      *sdjournal.Journal
	entries      *queue.MessageQueue
	stopchan     chan struct{}
	shutdownchan chan struct{}
	wgroup       *sync.WaitGroup
	logger       log15.Logger
}

type Converter func(map[string]string) model.FullMessage

func EntryToSyslog(entry map[string]string) model.ParsedMessage {
	m = model.Factory()
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

	return model.ParsedMessage{
		Client:         "journald",
		LocalPort:      0,
		UnixSocketPath: "",
		Fields:         m,
	}
}

func makeMapConverter(coding string) Converter {
	decoder := utils.SelectDecoder(coding)
	generator := utils.NewGenerator()
	return func(m map[string]string) model.FullMessage {
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
		return model.FullMessage{
			Uid:    generator.Uid(),
			Parsed: EntryToSyslog(dest),
		}
	}
}

func NewReader(logger log15.Logger) (JournaldReader, error) {
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

func (r *reader) Entries() *queue.MessageQueue {
	return r.entries
}

func (r *reader) wait() chan struct{} {
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

func (r *reader) Start() {
	r.stopchan = make(chan struct{})
	r.entries = queue.NewMessageQueue()

	r.wgroup.Add(1)
	go func() {
		defer func() {
			r.entries.Dispose()
			//close(r.entries)
			r.wgroup.Done()
		}()

		var err error
		var nb uint64
		var entry *sdjournal.JournalEntry
		converter := makeMapConverter("utf8")

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
					} else if nb == 0 {
						select {
						case <-r.shutdownchan:
							return
						default:
							break LoopGetEntries
						}
					} else {
						entry, err = r.journal.GetEntry()
						if err != nil {
							return
						} else {
							r.entries.Put(converter(entry.Fields))
						}
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

func (r *reader) Stop() {
	if r.stopchan != nil {
		close(r.stopchan)
		r.wgroup.Wait()
	}
}

func (r *reader) Shutdown() {
	close(r.shutdownchan)
	r.wgroup.Wait()
	if r.stopchan != nil {
		close(r.stopchan)
	}
	go func() {
		r.journal.Close()
	}()
}
