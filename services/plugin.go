package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

func NewNetworkPlugin(t string, stasher model.Stasher, binderHandle int, loggerHandle int, m *metrics.Metrics, l log15.Logger) *NetworkPlugin {
	s := &NetworkPlugin{t: t, stasher: stasher, binderHandle: binderHandle, loggerHandle: loggerHandle, metrics: m, logger: l}
	s.mu = &sync.Mutex{}
	return s
}

// NetworkPlugin launches and controls the TCP service
type NetworkPlugin struct {
	t            string
	syslogConfs  []*conf.SyslogConfig
	parserConfs  []conf.ParserConfig
	kafkaConf    *conf.KafkaConfig
	auditConf    *conf.AuditConfig
	binderHandle int
	loggerHandle int
	metrics      *metrics.Metrics
	logger       log15.Logger
	stasher      model.Stasher
	shutdown     chan struct{}
	stdin        io.WriteCloser
	mu           *sync.Mutex
	ExitError    int32
}

func (s *NetworkPlugin) Stop() {
	s.mu.Lock()
	select {
	case <-s.shutdown:
	default:
		if s.stdin != nil {
			s.stdin.Write([]byte("stop\n"))
		}
	}
	s.mu.Unlock()
}

func (s *NetworkPlugin) Shutdown() {
	s.mu.Lock()
	select {
	case <-s.shutdown:
	default:
		if s.stdin != nil {
			s.stdin.Write([]byte("shutdown\n"))
		}
	}
	s.mu.Unlock()
}

func (s *NetworkPlugin) WaitPluginShutdown() {
	<-s.shutdown
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

func (s *NetworkPlugin) Start(test bool) ([]*model.ListenerInfo, error) {
	infos := []*model.ListenerInfo{}
	s.shutdown = make(chan struct{})

	exe, err := sys.Executable()
	if err != nil {
		close(s.shutdown)
		return infos, err
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

	cmd := &exec.Cmd{
		Path:       exe,
		Stderr:     os.Stderr,
		ExtraFiles: files,
		Env:        envs,
	}
	s.mu.Lock()
	s.stdin, err = cmd.StdinPipe()
	s.mu.Unlock()

	if err != nil {
		close(s.shutdown)
		return infos, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		close(s.shutdown)
		return infos, err
	}

	args := []string{fmt.Sprintf("skewer-%s", s.t)}
	if test {
		args = append(args, "--test")
	}
	cmd.Args = args

	err = cmd.Start()
	if err != nil {
		close(s.shutdown)
		return infos, err
	}

	startedChan := make(chan error)

	var once sync.Once

	go func() {
		kill := false
		initialized := false

		defer func() {
			// we arrive here
			// 1/ when the plugin process has stopped (stdout is closed, so scanner returns)
			// or 2/ if scanner faces a formatting error
			// or 3/ if the plugin sent a badly JSON encoded message
			// or 4/ if the plugin sent an unexpected message
			s.logger.Debug("End of plugin", "type", s.t)
			if kill {
				s.logger.Crit("KIIIIIIILLLLLL", "type", s.t)
				s.mu.Lock()
				cmd.Process.Kill()
				s.mu.Unlock()
			}
			err := cmd.Wait() // reap the zombie
			if err != nil {
				s.logger.Error("Plugin exited with an error", "type", s.t, "error", err)
			}
			if !cmd.ProcessState.Success() {
				atomic.StoreInt32(&s.ExitError, 1)
			}
			close(s.shutdown)
		}()

		// read JSON encoded messages that the plugin is going to write on stdout
		scanner := bufio.NewScanner(stdout)
		scanner.Split(PluginSplit)
		for scanner.Scan() {
			b := scanner.Bytes()
			if bytes.HasPrefix(b, []byte("syslog ")) {
				m := &model.TcpUdpParsedMessage{}
				err := json.Unmarshal(b[7:], m)
				if !initialized {
					msg := "Plugin sent a syslog message before being initialized"
					s.logger.Error(msg)
					once.Do(func() { startedChan <- fmt.Errorf(msg); close(startedChan) })
					kill = true
					return
				} else if err == nil {
					s.stasher.Stash(m)
				} else {
					s.logger.Warn("Plugin sent a badly encoded JSON log line", "error", err)
					kill = true
					return
				}
			} else if bytes.HasPrefix(b, []byte("started ")) {
				err := json.Unmarshal(b[8:], &infos)
				if err == nil {
					initialized = true
					once.Do(func() { close(startedChan) })
				} else {
					s.logger.Warn("Plugin sent a badly encoded JSON listener info", "error", err)
					once.Do(func() { startedChan <- err; close(startedChan) })
					kill = true
					return
				}
			} else if bytes.HasPrefix(b, []byte("starterror ")) {
				err := fmt.Errorf(string(b[11:]))
				once.Do(func() { startedChan <- err; close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("syslogconferror ")) {
				err := fmt.Errorf(string(b[16:]))
				once.Do(func() { startedChan <- err; close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("parserconferror ")) {
				err := fmt.Errorf(string(b[16:]))
				once.Do(func() { startedChan <- err; close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("kafkaconferror ")) {
				err := fmt.Errorf(string(b[15:]))
				once.Do(func() { startedChan <- err; close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("auditconferror ")) {
				err := fmt.Errorf(string(b[15:]))
				once.Do(func() { startedChan <- err; close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("nolistenererror")) {
				err := fmt.Errorf("No listener")
				once.Do(func() { startedChan <- err; close(startedChan) })
			} else {
				err := fmt.Errorf("Unexpected message from plugin")
				s.logger.Error("Unexpected message from plugin", "message", string(b))
				once.Do(func() { startedChan <- err; close(startedChan) })
				kill = true
				return
			}
		}
		err := scanner.Err()
		if err != nil {
			once.Do(func() { startedChan <- err; close(startedChan) })
			s.logger.Error("Plugin scanner error", "error", err)
			kill = true
			return
		}
	}()

	scb, _ := json.Marshal(s.syslogConfs)
	pcb, _ := json.Marshal(s.parserConfs)
	kcb, _ := json.Marshal(s.kafkaConf)
	acb, _ := json.Marshal(s.auditConf)

	s.mu.Lock()
	s.stdin.Write([]byte("syslogconf "))
	s.stdin.Write(scb)
	s.stdin.Write([]byte("\n"))

	s.stdin.Write([]byte("parserconf "))
	s.stdin.Write(pcb)
	s.stdin.Write([]byte("\n"))

	s.stdin.Write([]byte("kafkaconf "))
	s.stdin.Write(kcb)
	s.stdin.Write([]byte("\n"))

	s.stdin.Write([]byte("auditconf "))
	s.stdin.Write(acb)
	s.stdin.Write([]byte("\n"))

	s.stdin.Write([]byte("start\n"))
	s.mu.Unlock()
	rerr := <-startedChan

	if rerr != nil {
		infos = nil
		s.logger.Debug("Ask the erred plugin to stop", "type", s.t)
		s.Shutdown()
		s.WaitPluginShutdown()
	}

	return infos, rerr
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
	b, err := json.Marshal(m)
	if err == nil {
		s := fmt.Sprintf("syslog %s", string(b))
		fmt.Fprintf(os.Stdout, "%010d %s\n", len(s), s)
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
	for scanner.Scan() {
		parts := strings.SplitN(strings.Trim(scanner.Text(), "\r\n "), " ", 2)
		command = parts[0]
		switch command {
		case "start":
			if p.syslogConfs == nil || p.parserConfs == nil || p.kafkaConf == nil || p.auditConf == nil {
				errs := fmt.Sprintf("syslogconferror syslog conf or parser conf was not provided to plugin")
				fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
				p.svc = nil
			} else {
				p.svc, cancel = NewNetworkService(typ, p, generator, binderClient, nil, logger)
				if p.svc == nil {
					errs := "starterror NewNetworkService returned nil"
					fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
				} else {
					p.svc.SetConf(p.syslogConfs, p.parserConfs)
					p.svc.SetKafkaConf(p.kafkaConf)
					p.svc.SetAuditConf(p.auditConf)
					infos, err := p.svc.Start(test)
					if err != nil {
						errs := fmt.Sprintf("starterror %s", err.Error())
						fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
						p.svc = nil
					} else if len(infos) == 0 && typ != "skewer-relp" && typ != "skewer-journal" && typ != "skewer-audit" {
						// (RELP, Journal and audit never report infos)
						p.svc.Stop()
						p.svc = nil
						errs := "nolistenererror"
						fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
					} else {
						infosb, _ := json.Marshal(infos)
						answer := fmt.Sprintf("started %s", infosb)
						fmt.Fprintf(os.Stdout, "%010d %s\n", len(answer), answer)
					}
				}
			}
		case "stop":
			if p.svc != nil {
				p.svc.Stop()
				p.svc.WaitClosed()
			}
		case "shutdown":
			if p.svc != nil {
				p.svc.Stop()
				p.svc.WaitClosed()
			}
			if cancel != nil {
				cancel()
				time.Sleep(400 * time.Millisecond) // give a chance for cleaning to be executed before plugin process ends
			}
			return nil
		case "syslogconf":
			args = parts[1]
			sc := []*conf.SyslogConfig{}
			err := json.Unmarshal([]byte(args), &sc)
			if err == nil {
				p.syslogConfs = sc
			} else {
				p.syslogConfs = nil
				errs := fmt.Sprintf("syslogconferror %s", err.Error())
				fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
			}
		case "parserconf":
			args = parts[1]
			pc := []conf.ParserConfig{}
			err := json.Unmarshal([]byte(args), &pc)
			if err == nil {
				p.parserConfs = pc
			} else {
				p.parserConfs = nil
				errs := fmt.Sprintf("parserconferror %s", err.Error())
				fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
			}
		case "kafkaconf":
			args = parts[1]
			kc := conf.KafkaConfig{}
			err := json.Unmarshal([]byte(args), &kc)
			if err == nil {
				p.kafkaConf = &kc
			} else {
				p.kafkaConf = nil
				errs := fmt.Sprintf("kafkaconferror %s", err.Error())
				fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
			}
		case "auditconf":
			args = parts[1]
			ac := conf.AuditConfig{}
			err := json.Unmarshal([]byte(args), &ac)
			if err == nil {
				p.auditConf = &ac
			} else {
				p.auditConf = nil
				errs := fmt.Sprintf("auditconferror %s", err.Error())
				fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
			}
		default:
			return fmt.Errorf("Unknown command")
		}

	}
	e := scanner.Err()
	if e != nil {
		logger.Error("In plugin, scanning stdin met error", "error", e)
		return e
	}
	return nil
}
