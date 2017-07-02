// +build linux

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
	j        *sdjournal.Journal
	Entries  chan map[string]string
stopchan chan bool
	wg       *sync.WaitGroup
}

func NewReader() (r *Reader, err error) {
r = &Reader{}
	r.j, err = sdjournal.NewJournal()
	if err != nil {
		return nil, err
	}
	err = r.j.SeekTail()
	if err != nil {
		r.j.Close()
		return nil, err
	}
	_, err = r.j.Previous()
	if err != nil {
		r.j.Close()
		return nil, err
	}
	r.Entries = make(chan map[string]string)
	r.wg = &sync.WaitGroup{}
	return r, nil
}

func (r *Reader) wait() chan int {
	events := make(chan int)
	r.wg.Add(1)

	go func() {
		defer r.wg.Done()
		var ev int

	WaitLoop:
		for {
			select {
			case <-r.stopchan:
				break WaitLoop
			default:
				ev = r.j.Wait(time.Second)
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
	r.wg.Add(1)

	go func() {
		defer r.wg.Done()
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
					nb, err = r.j.Next()
					if err != nil {
						// ???
					} else if nb == 0 {
						break LoopGetEntries
					} else {
						entry, err = r.j.GetEntry()
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
	r.wg.Wait()
}

func (r *Reader) Close() error {
	r.Stop()
	close(r.Entries)
	return r.j.Close()
}
