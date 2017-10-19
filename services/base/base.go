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
	SyslogConfigs   []conf.SyslogConfig
	ParserConfigs   []conf.ParserConfig
	Logger          log15.Logger
	Binder          *binder.BinderClient
	UnixSocketPaths []string
	Protocol        string
	Connections     map[io.Closer]bool
	QueueSize       uint64

	connMutex   *sync.Mutex
	statusMutex *sync.Mutex
	Pool        *sync.Pool
}

func (s *BaseService) Init() {
	s.UnixSocketPaths = []string{}
	s.connMutex = &sync.Mutex{}
	s.Connections = map[io.Closer]bool{}
	s.statusMutex = &sync.Mutex{}
}

func (s *BaseService) LockStatus() {
	s.statusMutex.Lock()
}

func (s *BaseService) UnlockStatus() {
	s.statusMutex.Unlock()
}

func (s *BaseService) SetConf(sc []conf.SyslogConfig, pc []conf.ParserConfig, queueSize uint64) {
	s.SyslogConfigs = sc
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
		conn.Close()
		delete(s.Connections, conn)
	}
	s.connMutex.Unlock()
}

func (s *BaseService) CloseConnections() {
	s.connMutex.Lock()
	for conn, _ := range s.Connections {
		conn.Close()
	}
	for _, path := range s.UnixSocketPaths {
		if !strings.HasPrefix(path, "@") {
			os.Remove(path)
		}
	}
	s.Connections = map[io.Closer]bool{}
	s.UnixSocketPaths = []string{}
	s.connMutex.Unlock()
}

func (s *BaseService) ClearConnections() {
	s.connMutex.Lock()
	s.Connections = map[io.Closer]bool{}
	s.connMutex.Unlock()
}
