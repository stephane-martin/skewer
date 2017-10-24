package base

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
)

var stdoutLock sync.Mutex

// SUCC = "SUCCESS" as byte array.
var SUCC = []byte("SUCCESS")
var SYSLOG = []byte("syslog")
var INFOS = []byte("infos")

// Wout writes a message to the controller via stdout
func Wout(header []byte, m []byte) (err error) {
	stdoutLock.Lock()
	err = utils.W(os.Stdout, header, m)
	stdoutLock.Unlock()
	return
}

// Reporter is used by plugins to report new syslog messages to the controller.
type Reporter struct {
	name   string
	logger log15.Logger
	pipe   *os.File
	queue  *queue.MessageQueue
}

// NewReporter creates a controller.
func NewReporter(name string, l log15.Logger, pipe *os.File) *Reporter {
	rep := &Reporter{name: name, logger: l, pipe: pipe}
	if pipe != nil {
		rep.queue = queue.NewMessageQueue()
		go rep.pushqueue()
	}
	return rep
}

func (s *Reporter) pushqueue() {
	var m *model.TcpUdpParsedMessage
	var b []byte
	var err error

	for s.queue.Wait(0) {
		m, err = s.queue.Get()
		if m != nil && err == nil {
			b, err = m.MarshalMsg(nil)
			if err == nil {
				_, err = s.pipe.Write(b)
				if err != nil {
					s.logger.Crit("Unexpected error when writing messages to the plugin pipe", "error", err)
					return
				}
			} else {
				s.logger.Warn("A message provided by a plugin could not be serialized", "error", err)
			}
		}
	}
}

// Stop stops the reporter.
func (s *Reporter) Stop() {
	if s.queue != nil {
		s.queue.Dispose()
	}
}

// Stash reports one syslog message to the controller.
func (s *Reporter) Stash(m model.TcpUdpParsedMessage) (fatal, nonfatal error) {
	if s.queue == nil {
		b, err := m.MarshalMsg(nil)
		if err != nil {
			// should not happen
			s.logger.Warn("A syslog message could not be serialized", "type", s.name, "error", err)
			return nil, err
		}
		err = Wout(SYSLOG, b)
		if err != nil {
			s.logger.Crit("Could not write message to upstream. There was message loss", "error", err, "type", s.name)
			return err, nil
		}
		return nil, nil
	}

	s.queue.Put(m)
	return nil, nil
}

// Report reports information about the actual listening ports to the controller.
func (s *Reporter) Report(infos []model.ListenerInfo) error {
	b, err := json.Marshal(infos)
	if err != nil {
		return err
	}
	return Wout(INFOS, b)
}
