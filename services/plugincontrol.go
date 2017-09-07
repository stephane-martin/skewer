package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

// NetworkPlugin launches and controls the plugins services
type NetworkPlugin struct {
	typ NetworkServiceType

	conf conf.BaseConfig

	binderHandle int
	loggerHandle int
	logger       log15.Logger
	stasher      model.Stasher

	metricsChan chan []*dto.MetricFamily
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	cmd         *exec.Cmd

	ShutdownChan chan struct{}
	stdinMu      *sync.Mutex
	startedMu    *sync.Mutex
	createdMu    *sync.Mutex
	started      bool
	created      bool
}

func NewNetworkPlugin(typ NetworkServiceType, stasher model.Stasher, binderHandle int, loggerHandle int, l log15.Logger) *NetworkPlugin {
	s := &NetworkPlugin{
		typ:          typ,
		stasher:      stasher,
		binderHandle: binderHandle,
		loggerHandle: loggerHandle,
		logger:       l,
		stdinMu:      &sync.Mutex{},
		startedMu:    &sync.Mutex{},
		createdMu:    &sync.Mutex{},
	}
	s.metricsChan = make(chan []*dto.MetricFamily)
	s.ShutdownChan = make(chan struct{})
	return s
}

func (s *NetworkPlugin) Gather() ([]*dto.MetricFamily, error) {
	select {
	case <-s.ShutdownChan:
		return []*dto.MetricFamily{}, nil
	default:
		s.startedMu.Lock()
		defer s.startedMu.Unlock()
		if s.started {
			s.stdinMu.Lock()
			if s.stdin != nil {
				utils.W(s.stdin, "gathermetrics", utils.NOW)
			} else {
				return []*dto.MetricFamily{}, nil
			}
			s.stdinMu.Unlock()
			select {
			case <-time.After(2 * time.Second):
				return []*dto.MetricFamily{}, nil
			case metrics, more := <-s.metricsChan:
				if more {
					return metrics, nil
				} else {
					return []*dto.MetricFamily{}, nil
				}
			}
		} else {
			return []*dto.MetricFamily{}, nil
		}
	}
}

func (s *NetworkPlugin) Stop() {
	select {
	case <-s.ShutdownChan:
	default:
		s.startedMu.Lock()
		s.stdinMu.Lock()
		defer func() {
			s.stdinMu.Unlock()
			s.startedMu.Unlock()
		}()
		if s.started {
			if s.stdin != nil {
				utils.W(s.stdin, "stop", utils.NOW)
			}
		}
	}
}

func (s *NetworkPlugin) Shutdown(killTimeOut time.Duration) {
	// in case the plugin was in fact never used...
	s.createdMu.Lock()
	if !s.created {
		s.createdMu.Unlock()
		return
	}
	s.createdMu.Unlock()

	select {
	case <-s.ShutdownChan:
		// the plugin is already dead
	default:
		// ask to shutdown
		s.stdinMu.Lock()
		if s.stdin != nil {
			utils.W(s.stdin, "shutdown", utils.NOW)
		}
		s.stdinMu.Unlock()

		// wait for plugin process termination
		if killTimeOut == 0 {
			<-s.ShutdownChan
		} else {
			select {
			case <-s.ShutdownChan:
			case <-time.After(killTimeOut):
				// after timeout kill the process
				s.stdinMu.Lock()
				s.cmd.Process.Kill()
				s.stdinMu.Unlock()
			}
		}
	}

}

func (s *NetworkPlugin) SetConf(c conf.BaseConfig) {
	s.conf = c
}

func (s *NetworkPlugin) Start() ([]*model.ListenerInfo, error) {
	s.startedMu.Lock()
	if s.started {
		s.startedMu.Unlock()
		return nil, fmt.Errorf("Plugin already started")
	}

	infos := []*model.ListenerInfo{}
	startErrorChan := make(chan error)
	var once sync.Once

	doKill := func() {
		s.logger.Crit("killing misbehaving plugin", "type", s.typ)
		s.stdinMu.Lock()
		s.cmd.Process.Kill()
		s.stdinMu.Unlock()
	}

	go func() {
		kill := false
		initialized := false

		defer func() {
			// we arrive here
			// 0/ when the plugin has been normally stopped
			// 1/ when the plugin process has been shut down (stdout is closed, so scanner returns)
			// or 2/ if scanner faces a formatting error
			// or 3/ if the plugin sent a badly JSON encoded message
			// or 4/ if the plugin sent an unexpected message
			s.logger.Debug("Plugin controller is stopping", "type", s.typ)
			once.Do(func() {
				startErrorChan <- fmt.Errorf("Unexpected end of plugin before it was initialized")
				close(startErrorChan)
			})

			if kill {
				doKill()
			} else {
				select {
				case <-s.ShutdownChan:
					// child has exited
					s.logger.Debug("End of plugin child process", "type", s.typ)
				default:
					// child is still alive
					s.startedMu.Lock()
					if s.started {
						// if the child had been inactive in a normal way,
						// it would have sent the "stopped" message first,
						// and s.started would be false
						// so we know that something is going wrong
						s.startedMu.Unlock()
						doKill()
					} else {
						s.startedMu.Unlock()
						s.logger.Debug("Plugin process has stopped and is inactive", "type", s.typ)
					}
				}
			}
		}() // end of defer

		// read JSON encoded messages that the plugin is going to write on stdout
		scanner := bufio.NewScanner(s.stdout)
		scanner.Split(utils.PluginSplit)
		var command string

		for scanner.Scan() {
			parts := strings.SplitN(scanner.Text(), " ", 2)
			command = parts[0]
			switch command {
			case "syslog":
				if len(parts) == 2 {
					if !initialized {
						msg := "Plugin sent a syslog message before being initialized"
						s.logger.Error(msg)
						once.Do(func() { startErrorChan <- fmt.Errorf(msg); close(startErrorChan) })
						kill = true
						return
					} else {
						m := &model.TcpUdpParsedMessage{}
						_, err := m.UnmarshalMsg([]byte(parts[1]))
						if err == nil {
							s.stasher.Stash(m)
						} else {
							s.logger.Warn("Plugin sent a badly encoded log line", "error", err)
							kill = true
							return
						}
					}
				}
			case "started":
				if len(parts) == 2 {
					err := json.Unmarshal([]byte(parts[1]), &infos)
					if err == nil {
						initialized = true
						once.Do(func() { close(startErrorChan) })
					} else {
						s.logger.Warn("Plugin sent a badly encoded JSON listener info", "error", err)
						once.Do(func() { startErrorChan <- err; close(startErrorChan) })
						kill = true
						return
					}
				}
			case "stopped":
				s.startedMu.Lock()
				s.started = false
				s.startedMu.Unlock()
				return
			case "shutdown":
				// plugin child is shutting down, eventually the scanner will return normally
			case "starterror":
				if len(parts) == 2 {
					err := fmt.Errorf(parts[1])
					once.Do(func() { startErrorChan <- err; close(startErrorChan) })
				}
			case "conferror":
				if len(parts) == 2 {
					err := fmt.Errorf(parts[1])
					once.Do(func() { startErrorChan <- err; close(startErrorChan) })
				}
			case "nolistenererror":
				err := fmt.Errorf("No listener")
				once.Do(func() { startErrorChan <- err; close(startErrorChan) })
			case "metrics":
				if len(parts) == 2 {
					families := []*dto.MetricFamily{}
					err := json.Unmarshal([]byte(parts[1]), &families)
					if err == nil {
						s.metricsChan <- families
					} else {
						// TODO
					}
				} else {
					// TODO
				}
			default:
				err := fmt.Errorf("Unexpected message from plugin")
				s.logger.Error("Unexpected message from plugin", "command", command)
				once.Do(func() { startErrorChan <- err; close(startErrorChan) })
				kill = true
				return
			}
		}
		err := scanner.Err()
		if err == nil {
			// scanner has returned without error
			// it means that the plugin child stdin has been closed
			// so we know that the plugin child has exited
			// let's wait that the shutdown channel has been closed before executing the defer()
			<-s.ShutdownChan
		} else {
			once.Do(func() { startErrorChan <- err; close(startErrorChan) })
			s.logger.Error("Plugin scanner error", "error", err)
			kill = true
			return
		}
	}()

	cb, _ := json.Marshal(s.conf)

	s.stdinMu.Lock()
	utils.W(s.stdin, "conf", cb)
	utils.W(s.stdin, "start", utils.NOW)
	s.stdinMu.Unlock()

	rerr := <-startErrorChan
	if rerr == nil {
		s.started = true
		s.startedMu.Unlock()
		return infos, nil
	} else {
		s.startedMu.Unlock()
		infos = nil
		s.Shutdown(time.Second)
		return infos, rerr
	}
}

func (s *NetworkPlugin) Create(test bool) error {
	s.createdMu.Lock()
	if s.created {
		s.createdMu.Unlock()
		return nil
	}

	s.ShutdownChan = make(chan struct{})

	exe, err := sys.Executable()
	if err != nil {
		return err
	}

	envs := []string{"PATH=/bin:/usr/bin"}
	files := []*os.File{}
	if s.binderHandle != 0 {
		files = append(files, os.NewFile(uintptr(s.binderHandle), "binder"))
		envs = append(envs, "HAS_BINDER=TRUE")
	}
	if s.loggerHandle != 0 {
		files = append(files, os.NewFile(uintptr(s.loggerHandle), "logger"))
		envs = append(envs, "HAS_LOGGER=TRUE")
	}

	s.cmd = &exec.Cmd{
		Path:       exe,
		Stderr:     os.Stderr,
		ExtraFiles: files,
		Env:        envs,
	}

	s.stdinMu.Lock()
	s.stdin, err = s.cmd.StdinPipe()
	s.stdinMu.Unlock()

	if err != nil {
		close(s.ShutdownChan)
		s.createdMu.Unlock()
		return err
	}

	s.stdout, err = s.cmd.StdoutPipe()
	if err != nil {
		close(s.ShutdownChan)
		s.createdMu.Unlock()
		return err
	}

	args := []string{ReverseNetworkServiceMap[s.typ]}
	if test {
		args = append(args, "--test")
	}
	s.cmd.Args = args

	err = s.cmd.Start()
	if err != nil {
		close(s.ShutdownChan)
		s.createdMu.Unlock()
		return err
	}
	s.created = true
	s.createdMu.Unlock()

	go func() {
		// monitor for plugin process termination
		s.cmd.Wait()
		close(s.ShutdownChan)
		s.createdMu.Lock()
		s.startedMu.Lock()
		s.created = false
		s.started = false
		s.startedMu.Unlock()
		s.createdMu.Unlock()
	}()

	return nil

}

type StorePlugin struct {
	*NetworkPlugin
}

func (s *StorePlugin) Stash(m *model.TcpUdpParsedMessage) {
	select {
	case <-s.ShutdownChan:
		return
	default:
		mb, err := m.MarshalMsg(nil)
		if err == nil {
			utils.W(s.stdin, "stash", mb)
		} else {
			// TODO
		}
	}
}

func NewStorePlugin(loggerHandle int, l log15.Logger) *StorePlugin {
	plug := NewNetworkPlugin(Store, nil, 0, loggerHandle, l)
	return &StorePlugin{NetworkPlugin: plug}
}
