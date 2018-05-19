package base

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"

	"github.com/awnumar/memguard"
	"github.com/gogo/protobuf/proto"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
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
	queue        *queue.BSliceQueue
	secret       *memguard.LockedBuffer
	pipeWriter   *utils.EncryptWriter
	pool         *sync.Pool
}

// NewReporter creates a reporter.
func NewReporter(name string, l log15.Logger, pipe *os.File) *Reporter {
	rep := Reporter{
		name:   name,
		logger: l,
		pipe:   pipe,
		pool: &sync.Pool{
			New: func() interface{} {
				return proto.NewBuffer(make([]byte, 0, 16384))
			},
		},
	}
	rep.bufferedPipe = bufio.NewWriter(pipe)
	return &rep
}

func (s *Reporter) getBuffer() (buf *proto.Buffer) {
	buf = s.pool.Get().(*proto.Buffer)
	buf.Reset()
	return buf
}

func (s *Reporter) putBuffer(buf *proto.Buffer) {
	if buf != nil {
		s.pool.Put(buf)
	}
}

func (s *Reporter) Start() {
	s.queue = queue.NewBSliceQueue()
	go s.pushqueue()
}

func (s *Reporter) SetSecret(secret *memguard.LockedBuffer) {
	s.secret = secret
	s.pipeWriter = utils.NewEncryptWriter(s.bufferedPipe, s.secret)
}

func (s *Reporter) pushqueue() {
	defer func() {
		_ = s.bufferedPipe.Flush()
		_ = s.pipe.Close()
	}()
	var m string
	var err error

	for s.queue.Wait(0) {
		for s.queue.Wait(100 * time.Millisecond) {
			_, m, err = s.queue.Get()
			if m != "" && err == nil {
				_, err = io.WriteString(s.pipeWriter, m)
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
	s.queue.Dispose()
}

// Stash reports one syslog message to the controller.
func (s *Reporter) Stash(m *model.FullMessage) error {
	var err error
	buf := s.getBuffer()
	defer s.putBuffer(buf)
	err = buf.Marshal(m)
	if err != nil {
		return eerrors.Wrapf(err, "Failed to marshal a message to be sent by plugin: %s", s.name)
	}
	return eerrors.Fatal(
		eerrors.Wrapf(
			s.queue.PutSlice(string(buf.Bytes())),
			"Failed to enqueue a message to be sent by plugin: %s", s.name,
		),
	)
}

// Report reports information about the actual listening ports to the controller.
func (s *Reporter) Report(infos []model.ListenerInfo) error {
	b, err := json.Marshal(infos)
	if err != nil {
		return eerrors.Wrapf(err, "Plugin '%s' failed to report infos", s.name)
	}
	stdoutLock.Lock()
	err = eerrors.Wrapf(
		utils.NewEncryptWriter(os.Stdout, nil).WriteWithHeader(INFOS, b),
		"Plugin '%s' failed to write infos to its stdout",
		s.name,
	)
	stdoutLock.Unlock()
	return err
}
