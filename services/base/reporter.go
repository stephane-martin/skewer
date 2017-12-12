package base

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
)

var stdoutLock sync.Mutex

var SUCC = []byte("SUCCESS")
var SYSLOG = []byte("syslog")
var INFOS = []byte("infos")
var SP = []byte(" ")

// Reporter is used by plugins to report new syslog messages to the controller.
type Reporter struct {
	name         string
	logger       log15.Logger
	pipe         *os.File
	bufferedPipe *bufio.Writer
	queue        *queue.MessageQueue
	secret       *memguard.LockedBuffer
	stdoutWriter *utils.EncryptWriter
	pipeWriter   *utils.EncryptWriter
}

// NewReporter creates a reporter.
func NewReporter(name string, l log15.Logger, pipe *os.File) *Reporter {
	rep := Reporter{
		name:   name,
		logger: l,
		pipe:   pipe,
	}
	if pipe != nil {
		rep.bufferedPipe = bufio.NewWriter(pipe)
	}
	return &rep
}

func (s *Reporter) Start() {
	if s.pipe != nil {
		s.queue = queue.NewMessageQueue()
		go s.pushqueue()
	}
}

func (s *Reporter) SetSecret(secret *memguard.LockedBuffer) {
	s.secret = secret
	s.stdoutWriter = utils.NewEncryptWriter(os.Stdout, s.secret)
	if s.pipe != nil {
		s.pipeWriter = utils.NewEncryptWriter(s.bufferedPipe, s.secret)
	}
}

func (s *Reporter) pushqueue() {
	defer func() {
		_ = s.bufferedPipe.Flush()
		_ = s.pipe.Close()
	}()
	var m *model.FullMessage
	var b []byte
	var err error

	for s.queue.Wait(0) {
		for s.queue.Wait(100 * time.Millisecond) {
			m, err = s.queue.Get()
			if m != nil && err == nil {
				b, err = m.MarshalMsg(nil)
				if err != nil {
					// should not happen
					s.logger.Warn("A syslog message could not be serialized", "type", s.name, "error", err)
					return
				}
				_, err = s.pipeWriter.Write(b)
				if err != nil {
					s.logger.Crit("Unexpected error when writing messages to the plugin pipe", "error", err)
					return
				}
			}
		}
		err = s.bufferedPipe.Flush()
		if err != nil {
			s.logger.Crit("Unexpected error when flushing the plugin pipe", "error", err)
			return
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
func (s *Reporter) Stash(m model.FullMessage) (fatal, nonfatal error) {
	if s.queue == nil {
		b, err := m.MarshalMsg(nil)
		if err != nil {
			// should not happen
			s.logger.Warn("A syslog message could not be serialized", "type", s.name, "error", err)
			return nil, err
		}
		stdoutLock.Lock()
		err = s.stdoutWriter.WriteWithHeader(SYSLOG, b)
		stdoutLock.Unlock()
		if err != nil {
			s.logger.Crit("Could not write message to upstream. There was message loss", "error", err, "type", s.name)
			return err, nil
		}
		return nil, nil
	}

	fatal = s.queue.Put(m) // fatal is set when the queue has been disposed
	return fatal, nil
}

// Report reports information about the actual listening ports to the controller.
func (s *Reporter) Report(infos []model.ListenerInfo) error {
	b, err := json.Marshal(infos)
	if err != nil {
		return err
	}
	stdoutLock.Lock()
	err = utils.NewEncryptWriter(os.Stdout, nil).WriteWithHeader(INFOS, b)
	stdoutLock.Unlock()
	return err
}
