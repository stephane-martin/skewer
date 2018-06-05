package base

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"sync"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/reservoir"
	"github.com/stephane-martin/skewer/utils/waiter"
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
	reserv       *reservoir.Reservoir
	secret       *memguard.LockedBuffer
	pipeWriter   *utils.EncryptWriter
}

// NewReporter creates a reporter.
func NewReporter(name string, l log15.Logger, pipe *os.File) *Reporter {
	rep := Reporter{
		name:   name,
		logger: l,
		pipe:   pipe,
		reserv: reservoir.NewReservoir(5000),
	}
	rep.bufferedPipe = bufio.NewWriterSize(pipe, 32768)
	return &rep
}

func (s *Reporter) Start() {
	go s.pushqueue()
}

func (s *Reporter) SetSecret(secret *memguard.LockedBuffer) {
	s.secret = secret
	s.pipeWriter = utils.NewEncryptWriter(s.bufferedPipe, s.secret)
}

func (s *Reporter) pushqueue() {
	defer func() {
		s.bufferedPipe.Flush()
		s.pipe.Close()
	}()

	m := make(map[utils.MyULID]string, 5000)
	w := waiter.Default()

	for {
		err := s.reserv.DeliverTo(m)
		if err == eerrors.ErrQDisposed {
			return
		}

		if len(m) == 0 {
			w.Wait()
			continue
		}
		w.Reset()

		for _, v := range m {
			_, err := io.WriteString(s.pipeWriter, v)
			if err != nil {
				s.logger.Crit("Unexpected error when writing messages to the plugin pipe", "error", err)
				return
			}
		}
		err = s.bufferedPipe.Flush()

		for k := range m {
			delete(m, k)
		}

		if err != nil {
			s.logger.Crit("Unexpected error when flushing the plugin pipe", "error", err)
			return
		}
	}
}

// Stop stops the reporter.
func (s *Reporter) Stop() {
	s.reserv.Dispose()
}

// Stash reports one syslog message to the controller.
func (s *Reporter) Stash(m *model.FullMessage) error {
	err := s.reserv.AddMessage(m)
	if err != nil {
		return eerrors.Wrapf(err, "Failed to marshal a message to be sent by plugin: %s", s.name)
	}
	return nil
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
