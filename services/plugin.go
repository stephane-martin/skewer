package services

import (
	"bufio"
	"context"
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

func w(dest io.Writer, header string, message []byte) {
	l := len(header) + len(message) + 1
	fmt.Fprintf(dest, "%010d ", l)
	dest.Write([]byte(header))
	dest.Write([]byte(" "))
	dest.Write(message)
	dest.Write([]byte("\n"))
}

var NOW []byte = []byte("now")

func NewNetworkPlugin(t string, stasher model.Stasher, binderHandle int, loggerHandle int, l log15.Logger) *NetworkPlugin {
	s := &NetworkPlugin{
		t:            t,
		stasher:      stasher,
		binderHandle: binderHandle,
		loggerHandle: loggerHandle,
		logger:       l,
		stdinMu:      &sync.Mutex{},
		startedMu:    &sync.Mutex{},
		createdMu:    &sync.Mutex{},
	}
	s.metricsChan = make(chan []*dto.MetricFamily)
	s.shutdown = make(chan struct{})
	return s
}

// NetworkPlugin launches and controls the TCP service
type NetworkPlugin struct {
	t string

	syslogConfs []*conf.SyslogConfig
	parserConfs []conf.ParserConfig
	kafkaConf   *conf.KafkaConfig
	auditConf   *conf.AuditConfig

	binderHandle int
	loggerHandle int
	logger       log15.Logger
	stasher      model.Stasher

	metricsChan chan []*dto.MetricFamily
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	cmd         *exec.Cmd

	shutdown  chan struct{}
	stdinMu   *sync.Mutex
	startedMu *sync.Mutex
	createdMu *sync.Mutex
	started   bool
	created   bool
}

func (s *NetworkPlugin) Gather() ([]*dto.MetricFamily, error) {
	select {
	case <-s.shutdown:
		return []*dto.MetricFamily{}, nil
	default:
		s.startedMu.Lock()
		defer s.startedMu.Unlock()
		if s.started {
			s.stdinMu.Lock()
			if s.stdin != nil {
				w(s.stdin, "gathermetrics", NOW)
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
	case <-s.shutdown:
	default:
		s.startedMu.Lock()
		s.stdinMu.Lock()
		defer func() {
			s.stdinMu.Unlock()
			s.startedMu.Unlock()
		}()
		if s.started {
			if s.stdin != nil {
				w(s.stdin, "stop", NOW)
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
	case <-s.shutdown:
		// the plugin is already dead
	default:
		// ask to shutdown
		s.stdinMu.Lock()
		if s.stdin != nil {
			w(s.stdin, "shutdown", NOW)
		}
		s.stdinMu.Unlock()

		// wait for plugin process termination
		if killTimeOut == 0 {
			<-s.shutdown
		} else {
			select {
			case <-s.shutdown:
			case <-time.After(killTimeOut):
				// after timeout kill the process
				s.stdinMu.Lock()
				s.cmd.Process.Kill()
				s.stdinMu.Unlock()
			}
		}
	}

}

func (s *NetworkPlugin) SetConf(sc []*conf.SyslogConfig, pc []conf.ParserConfig) {
	s.syslogConfs = sc
	s.parserConfs = pc
}

func (s *NetworkPlugin) SetKafkaConf(kc *conf.KafkaConfig) {
	s.kafkaConf = kc
}

func (s *NetworkPlugin) SetAuditConf(ac *conf.AuditConfig) {
	s.auditConf = ac
}

func (s *NetworkPlugin) Start() ([]*model.ListenerInfo, error) {
	s.startedMu.Lock()
	if s.started {
		return nil, fmt.Errorf("Plugin already started")
	}

	infos := []*model.ListenerInfo{}
	startErrorChan := make(chan error)
	var once sync.Once

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
			s.logger.Debug("End of plugin controller", "type", s.t)
			s.createdMu.Lock()
			s.startedMu.Lock()

			if kill {
				s.logger.Crit("killing misbehaving plugin", "type", s.t)
				s.stdinMu.Lock()
				s.cmd.Process.Kill()
				s.stdinMu.Unlock()
				s.cmd.Wait()
				s.created = false
				s.started = false
				close(s.shutdown)
				s.startedMu.Unlock()
				s.createdMu.Unlock()
				return
			} else if !s.created {
				// plugin process hasd exited normally (typically because we asked for it)
				s.logger.Debug("Normal end of plugin process", "type", s.t)
				s.cmd.Wait()
				s.created = false
				s.started = false
				close(s.shutdown)
				s.startedMu.Unlock()
				s.createdMu.Unlock()
				return
			} else if !s.started {
				// normal stop, the plugin process continues to live
				s.logger.Debug("Plugin process has stopped and is inactive", "type", s.t)
				s.startedMu.Unlock()
				s.createdMu.Unlock()
				return
			} else {
				s.logger.Crit("killing misbehaving plugin", "type", s.t)
				s.stdinMu.Lock()
				s.cmd.Process.Kill()
				s.stdinMu.Unlock()
				s.cmd.Wait()
				s.created = false
				s.started = false
				close(s.shutdown)
				s.startedMu.Unlock()
				s.createdMu.Unlock()
				return
			}
		}()

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
				s.createdMu.Lock()
				s.startedMu.Lock()
				s.started = false
				s.created = false
				s.startedMu.Unlock()
				s.createdMu.Unlock()
				return
			case "starterror":
				if len(parts) == 2 {
					err := fmt.Errorf(parts[1])
					once.Do(func() { startErrorChan <- err; close(startErrorChan) })
				}
			case "syslogconferror":
				if len(parts) == 2 {
					err := fmt.Errorf(parts[1])
					once.Do(func() { startErrorChan <- err; close(startErrorChan) })
				}
			case "parserconferror":
				if len(parts) == 2 {
					err := fmt.Errorf(parts[1])
					once.Do(func() { startErrorChan <- err; close(startErrorChan) })
				}
			case "kafkaconferror":
				if len(parts) == 2 {
					err := fmt.Errorf(parts[1])
					once.Do(func() { startErrorChan <- err; close(startErrorChan) })
				}
			case "auditconferror":
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
		if err != nil {
			once.Do(func() { startErrorChan <- err; close(startErrorChan) })
			s.logger.Error("Plugin scanner error", "error", err)
			kill = true
			return
		}
	}()

	scb, _ := json.Marshal(s.syslogConfs)
	pcb, _ := json.Marshal(s.parserConfs)
	kcb, _ := json.Marshal(s.kafkaConf)
	acb, _ := json.Marshal(s.auditConf)

	s.stdinMu.Lock()
	w(s.stdin, "syslogconf", scb)
	w(s.stdin, "parserconf", pcb)
	w(s.stdin, "kafkaconf", kcb)
	w(s.stdin, "auditconf", acb)
	w(s.stdin, "start", NOW)
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
	defer s.createdMu.Unlock()
	if s.created {
		return nil
	}

	s.shutdown = make(chan struct{})

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
		close(s.shutdown)
		return err
	}

	s.stdout, err = s.cmd.StdoutPipe()
	if err != nil {
		close(s.shutdown)
		return err
	}

	args := []string{fmt.Sprintf("skewer-%s", s.t)}
	if test {
		args = append(args, "--test")
	}
	s.cmd.Args = args

	err = s.cmd.Start()
	if err != nil {
		close(s.shutdown)
		return err
	}
	s.created = true
	return nil

}

// NetworkPluginProvider implements the TCP service in a separated process
type NetworkPluginProvider struct {
	svc         NetworkService
	logger      log15.Logger
	syslogConfs []*conf.SyslogConfig
	parserConfs []conf.ParserConfig
	kafkaConf   *conf.KafkaConfig
	auditConf   *conf.AuditConfig
}

func (p *NetworkPluginProvider) Stash(m *model.TcpUdpParsedMessage) {
	b, err := m.MarshalMsg(nil)
	if err == nil {
		w(os.Stdout, "syslog", b)
	} else {
		// should not happen
		p.logger.Warn("In plugin, a syslog message could not be serialized to JSON ?!")
	}
}

func (p *NetworkPluginProvider) Launch(typ string, test bool, binderClient *sys.BinderClient, logger log15.Logger) error {
	generator := utils.Generator(context.Background(), logger)
	p.logger = logger

	var scanner *bufio.Scanner
	var command string
	var args string
	var cancel context.CancelFunc

	scanner = bufio.NewScanner(os.Stdin)
	scanner.Split(utils.PluginSplit)

	for scanner.Scan() {
		parts := strings.SplitN(strings.Trim(scanner.Text(), "\r\n "), " ", 2)
		command = parts[0]
		switch command {
		case "start":
			if p.syslogConfs == nil || p.parserConfs == nil || p.kafkaConf == nil || p.auditConf == nil {
				w(os.Stdout, "syslogconferror", []byte("syslog conf or parser conf was not provided to plugin"))
				p.svc = nil
				// TODO: return
			} else {
				p.svc, cancel = NewNetworkService(typ, p, generator, binderClient, logger)
				if p.svc == nil {
					w(os.Stdout, "starterror", []byte("NewNetworkService returned nil"))
				} else {
					p.svc.SetConf(p.syslogConfs, p.parserConfs)
					p.svc.SetKafkaConf(p.kafkaConf)
					p.svc.SetAuditConf(p.auditConf)
					infos, err := p.svc.Start(test)
					if err != nil {
						w(os.Stdout, "starterror", []byte(err.Error()))
						p.svc = nil
					} else if len(infos) == 0 && typ != "skewer-relp" && typ != "skewer-journal" && typ != "skewer-audit" {
						// (RELP, Journal and audit never report info about listening ports)
						p.svc.Stop()
						p.svc = nil
						w(os.Stdout, "nolistenererror", []byte("plugin is inactive"))
					} else {
						infosb, _ := json.Marshal(infos)
						w(os.Stdout, "started", infosb)
					}
				}
			}
		case "stop":
			if p.svc != nil {
				p.svc.Stop()
				p.svc.WaitClosed()

			}
			w(os.Stdout, "stopped", []byte("success"))
			// at the end of the stop return, we *do not return*. So the
			// plugin process continues to listenn for subsequent commands
		case "shutdown":
			if p.svc != nil {
				p.svc.Stop()
				p.svc.WaitClosed()
			}
			if cancel != nil {
				cancel()
				time.Sleep(400 * time.Millisecond) // give a chance for cleaning to be executed before plugin process ends
			}
			w(os.Stdout, "shutdown", []byte("success"))
			// at the end of shutdown command, we *return*. And the plugin process stops.
			return nil
		case "syslogconf":
			args = parts[1]
			sc := []*conf.SyslogConfig{}
			err := json.Unmarshal([]byte(args), &sc)
			if err == nil {
				p.syslogConfs = sc
			} else {
				p.syslogConfs = nil
				w(os.Stdout, "syslogconferror", []byte(err.Error()))
			}
		case "parserconf":
			args = parts[1]
			pc := []conf.ParserConfig{}
			err := json.Unmarshal([]byte(args), &pc)
			if err == nil {
				p.parserConfs = pc
			} else {
				p.parserConfs = nil
				w(os.Stdout, "parserconferror", []byte(err.Error()))
			}
		case "kafkaconf":
			args = parts[1]
			kc := conf.KafkaConfig{}
			err := json.Unmarshal([]byte(args), &kc)
			if err == nil {
				p.kafkaConf = &kc
			} else {
				p.kafkaConf = nil
				w(os.Stdout, "kafkaconferror", []byte(err.Error()))
			}
		case "auditconf":
			args = parts[1]
			ac := conf.AuditConfig{}
			err := json.Unmarshal([]byte(args), &ac)
			if err == nil {
				p.auditConf = &ac
			} else {
				p.auditConf = nil
				w(os.Stdout, "auditconferror", []byte(err.Error()))
			}
		case "gathermetrics":
			families, err := p.svc.Gather()
			if err != nil {
				// TODO
			}
			familiesb, err := json.Marshal(families)
			if err == nil {
				w(os.Stdout, "metrics", familiesb)
			} else {
				// TODO
			}

		default:
			return fmt.Errorf("Unknown command")
		}

	}
	e := scanner.Err()
	if e != nil {
		logger.Error("In plugin controller, scanning stdin met error", "error", e)
		return e
	}
	return nil
}
