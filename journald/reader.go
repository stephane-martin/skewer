// +build linux,!nonsystemd

package journald

import (
	"context"
	"sync"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/inconshreveable/log15"
)

var Supported bool = true

type JournaldReader interface {
	Start()
	Stop()
	Entries() chan map[string]string
}

type reader struct {
	journal  *sdjournal.Journal
	entries  chan map[string]string
	stopchan chan bool
	wgroup   *sync.WaitGroup
	logger   log15.Logger
}

func NewReader(ctx context.Context, logger log15.Logger) (JournaldReader, error) {
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
	r.entries = make(chan map[string]string)
	r.wgroup = &sync.WaitGroup{}

	go func() {
		<-ctx.Done()
		r.Stop()
		close(r.entries)
		r.journal.Close()
	}()

	return r, nil
}

func (r *reader) Entries() chan map[string]string {
	return r.entries
}

func (r *reader) wait() chan int {
	events := make(chan int)
	r.wgroup.Add(1)

	go func() {
		defer r.wgroup.Done()
		var ev int

	WaitLoop:
		for {
			select {
			case <-r.stopchan:
				break WaitLoop
			default:
				ev = r.journal.Wait(time.Second)
				if ev == sdjournal.SD_JOURNAL_APPEND || ev == sdjournal.SD_JOURNAL_INVALIDATE {
					events <- ev
					close(events)
					break WaitLoop
				}
			}
		}
	}()

	return events
}

func (r *reader) Start() {
	r.stopchan = make(chan bool)
	r.wgroup.Add(1)

	go func() {
		defer r.wgroup.Done()
		var err error
		var nb uint64
		var entry *sdjournal.JournalEntry
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
						r.logger.Warn("journal.Next() error", "error", err)
					} else if nb == 0 {
						break LoopGetEntries
					} else {
						entry, err = r.journal.GetEntry()
						if err != nil {
							r.logger.Warn("journal.GetEntry() error", "error", err)
						} else {
							r.entries <- entry.Fields
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
	}
	r.wgroup.Wait()
}
