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

func W(header string, message []byte) (err error) {
	stdoutLock.Lock()
	err = utils.W(os.Stdout, header, message)
	stdoutLock.Unlock()
	return err
}

type Reporter struct {
	Name   string
	Logger log15.Logger
}

func (s *Reporter) StashMany(msgs []*model.TcpUdpParsedMessage) (fatal error, nonfatal error) {
	// when the plugin *produces* syslog messages, write them to stdout
	var b []byte
	var err error
	var m *model.TcpUdpParsedMessage

	stdoutLock.Lock()
	for _, m = range msgs {
		b, err = m.MarshalMsg(nil)
		if err == nil {
			err = utils.W(os.Stdout, "syslog", b)
			if err != nil {
				s.Logger.Crit("Could not write message to upstream. There was message loss", "error", err, "type", s.Name)
				stdoutLock.Unlock()
				return err, nil
			}
		} else {
			s.Logger.Warn("A syslog message could not be serialized", "type", s.Name, "error", err)
		}
	}
	stdoutLock.Unlock()
	return nil, nil
}

func (s *Reporter) Stash(m model.TcpUdpParsedMessage) (fatal error, nonfatal error) {
	// when the plugin *produces* a syslog message, write it to stdout
	b, err := m.MarshalMsg(nil)
	if err == nil {
		err = W("syslog", b)
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
