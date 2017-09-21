// +build linux,!nonsystemd

package journald

import (
	"sync"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/utils"
)

var Supported bool = true

type JournaldReader interface {
	Start(coding string)
	Stop()
	Shutdown()
	Entries() chan map[string]string
}

type reader struct {
	journal      *sdjournal.Journal
	entries      chan map[string]string
	stopchan     chan struct{}
	shutdownchan chan struct{}
	wgroup       *sync.WaitGroup
	logger       log15.Logger
}

type Converter func(map[string]string) map[string]string

func makeMapConverter(coding string) Converter {
	decoder := utils.SelectDecoder(coding)
	return func(m map[string]string) map[string]string {
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
		return dest
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

func (r *reader) Entries() chan map[string]string {
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

func (r *reader) Start(coding string) {
	r.stopchan = make(chan struct{})
	r.entries = make(chan map[string]string)

	r.wgroup.Add(1)
	go func() {
		defer func() {
			close(r.entries)
			r.wgroup.Done()
		}()

		var err error
		var nb uint64
		var entry *sdjournal.JournalEntry
		converter := makeMapConverter(coding)

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
							r.entries <- converter(entry.Fields)
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
