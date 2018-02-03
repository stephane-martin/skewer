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

type Stasher interface {
	Start()
	SetSecret(secret *memguard.LockedBuffer)
	Stop()
	Stash(m *model.FullMessage) (error, error)
}

type Reporter interface {
	Stasher
	Report(infos []model.ListenerInfo) error
}

// ReporterImpl is used by plugins to report new syslog messages to the controller.
type ReporterImpl struct {
	name         string
	logger       log15.Logger
	pipe         *os.File
	bufferedPipe *bufio.Writer
	queue        *queue.BSliceQueue
	secret       *memguard.LockedBuffer
	pipeWriter   *utils.EncryptWriter
	pool         *sync.Pool
}

// NewReporter creates a reporter.
func NewReporter(name string, l log15.Logger, pipe *os.File) *ReporterImpl {
	rep := ReporterImpl{
		name:   name,
		logger: l,
		pipe:   pipe,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 4096)
			},
		},
	}
	rep.bufferedPipe = bufio.NewWriter(pipe)
	return &rep
}

func (s *ReporterImpl) Start() {
	s.queue = queue.NewBSliceQueue()
	go s.pushqueue()
}

func (s *ReporterImpl) SetSecret(secret *memguard.LockedBuffer) {
	s.secret = secret
	s.pipeWriter = utils.NewEncryptWriter(s.bufferedPipe, s.secret)
}

func (s *ReporterImpl) pushqueue() {
	defer func() {
		_ = s.bufferedPipe.Flush()
		_ = s.pipe.Close()
	}()
	var m []byte
	var err error

	for s.queue.Wait(0) {
		for s.queue.Wait(100 * time.Millisecond) {
			_, m, err = s.queue.Get()
			if m != nil && err == nil {
				_, err = s.pipeWriter.Write(m)
				if cap(m) == 4096 {
					s.pool.Put(m)
				}
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
func (s *ReporterImpl) Stop() {
	s.queue.Dispose()
}

// Stash reports one syslog message to the controller.
func (s *ReporterImpl) Stash(m *model.FullMessage) (fatal, nonfatal error) {
	var b []byte
	var err error
	size := m.Size()
	if size > 4096 {
		b, err = m.Marshal()
	} else {
		b = s.pool.Get().([]byte)[:size]
		_, err = m.MarshalTo(b)
	}
	if err != nil {
		return nil, err
	}
	fatal = s.queue.PutSlice(b) // fatal is set when the queue has been disposed
	return fatal, nil
}

// Report reports information about the actual listening ports to the controller.
func (s *ReporterImpl) Report(infos []model.ListenerInfo) error {
	b, err := json.Marshal(infos)
	if err != nil {
		return err
	}
	stdoutLock.Lock()
	err = utils.NewEncryptWriter(os.Stdout, nil).WriteWithHeader(INFOS, b)
	stdoutLock.Unlock()
	return err
}
