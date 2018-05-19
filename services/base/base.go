package base

import (
	"io"
	"os"
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/sys/binder"
)

type BaseService struct {
	ParserConfigs   []conf.ParserConfig
	Logger          log15.Logger
	Binder          binder.Client
	UnixSocketPaths []string
	Connections     map[io.Closer]bool
	QueueSize       uint64

	connMutex   sync.Mutex
	statusMutex sync.Mutex
}

func (s *BaseService) Init() {
	s.UnixSocketPaths = []string{}
	s.Connections = map[io.Closer]bool{}
}

func (s *BaseService) LockStatus() {
	s.statusMutex.Lock()
}

func (s *BaseService) UnlockStatus() {
	s.statusMutex.Unlock()
}

func (s *BaseService) SetConf(pc []conf.ParserConfig, queueSize uint64) {
	s.ParserConfigs = pc
	s.QueueSize = queueSize
}

func (s *BaseService) AddConnection(conn io.Closer) {
	s.connMutex.Lock()
	s.Connections[conn] = true
	s.connMutex.Unlock()
}

func (s *BaseService) RemoveConnection(conn io.Closer) {
	s.connMutex.Lock()
	if _, ok := s.Connections[conn]; ok {
		_ = conn.Close()
		delete(s.Connections, conn)
	}
	s.connMutex.Unlock()
}

func (s *BaseService) CloseConnections() {
	s.connMutex.Lock()
	for conn, _ := range s.Connections {
		_ = conn.Close()
		delete(s.Connections, conn)
	}
	for _, path := range s.UnixSocketPaths {
		if !strings.HasPrefix(path, "@") {
			err := os.Remove(path)
			if err != nil {
				s.Logger.Warn("Error deleting unix socket path", "error", err)
			}
		}
	}
	s.Connections = map[io.Closer]bool{}
	s.UnixSocketPaths = []string{}
	s.connMutex.Unlock()
}

func (s *BaseService) ClearConnections() {
	s.connMutex.Lock()
	s.Connections = map[io.Closer]bool{}
	s.UnixSocketPaths = []string{}
	s.connMutex.Unlock()
}
