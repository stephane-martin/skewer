// +build linux,!nonsystemd

package journald

import (
	"sync"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
)

func Dummy() bool {
	return false
}

type Reader struct {
	journal  *sdjournal.Journal
	Entries  chan map[string]string
	stopchan chan bool
	wgroup   *sync.WaitGroup
}

func NewReader() (r *Reader, err error) {
	r = &Reader{}
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
	r.Entries = make(chan map[string]string)
	r.wgroup = &sync.WaitGroup{}
	return r, nil
}

func (r *Reader) wait() chan int {
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

func (r *Reader) Start() {
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
						// ???
					} else if nb == 0 {
						break LoopGetEntries
					} else {
						entry, err = r.journal.GetEntry()
						if err != nil {
							// ???
						}
						r.Entries <- entry.Fields
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
	}
	r.wgroup.Wait()
}

func (r *Reader) Close() error {
	r.Stop()
	close(r.Entries)
	return r.journal.Close()
}
