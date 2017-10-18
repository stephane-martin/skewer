package base

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

var stdoutLock sync.Mutex

var SUCC []byte = []byte("SUCCESS")

func W(header string, m []byte) (err error) {
	stdoutLock.Lock()
	err = utils.W(os.Stdout, header, m)
	stdoutLock.Unlock()
	return
}

type Reporter struct {
	Name   string
	Logger log15.Logger
	Pipe   *os.File
}

func (s *Reporter) W(b []byte) error {
	if s.Pipe == nil {
		return W("syslog", b)
	} else {
		_, err := s.Pipe.Write(b)
		return err
	}
}

func (s *Reporter) StashMany(msgs []*model.TcpUdpParsedMessage) (fatal error, nonfatal error) {
	// when the plugin *produces* syslog messages, write them to stdout
	var b []byte
	var err error
	var m *model.TcpUdpParsedMessage

	for _, m = range msgs {
		b, err = m.MarshalMsg(nil)
		if err == nil {
			err = s.W(b)
			if err != nil {
				s.Logger.Crit("Could not write message to upstream. There was message loss", "error", err, "type", s.Name)
				return err, nil
			}
		} else {
			s.Logger.Warn("A syslog message could not be serialized", "type", s.Name, "error", err)
		}
	}
	return nil, nil
}

func (s *Reporter) Stash(m model.TcpUdpParsedMessage) (fatal error, nonfatal error) {
	// when the plugin *produces* a syslog message, write it to stdout
	b, err := m.MarshalMsg(nil)
	if err == nil {
		err = s.W(b)
		if err != nil {
			s.Logger.Crit("Could not write message to upstream. There was message loss", "error", err, "type", s.Name)
			return err, nil
		} else {
			return nil, nil
		}
	} else {
		// should not happen
		s.Logger.Warn("A syslog message could not be serialized", "type", s.Name, "error", err)
		return nil, err
	}
}

func (s *Reporter) Report(infos []model.ListenerInfo) error {
	b, err := json.Marshal(infos)
	if err != nil {
		return err
	}
	return W("infos", b)
}
