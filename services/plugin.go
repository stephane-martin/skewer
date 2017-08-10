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

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

func NewNetworkPlugin(t string, stasher model.Stasher, binderHandle int, loggerHandle int, m *metrics.Metrics, l log15.Logger) NetworkService {
	return &NetworkPlugin{t: t, stasher: stasher, binderHandle: binderHandle, loggerHandle: loggerHandle, metrics: m, logger: l}
}

// NetworkPlugin launches and controls the TCP service
type NetworkPlugin struct {
	t            string
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	syslogConfs  []*conf.SyslogConfig
	parserConfs  []conf.ParserConfig
	kafkaConf    *conf.KafkaConfig
	binderHandle int
	loggerHandle int
	metrics      *metrics.Metrics
	logger       log15.Logger
	stasher      model.Stasher
}

func (s *NetworkPlugin) Stop() {
	if s.stdin != nil {
		s.stdin.Write([]byte("stop\n"))
	}
}

func (s *NetworkPlugin) WaitClosed() {
	// TODO: kill the plugin if it has not stopped after a few seconds
	if s.cmd != nil {
		s.cmd.Wait()
		s.cmd = nil
		s.stdin = nil
	}
}

func (s *NetworkPlugin) SetConf(sc []*conf.SyslogConfig, pc []conf.ParserConfig) {
	s.syslogConfs = sc
	s.parserConfs = pc
}

func (s *NetworkPlugin) SetKafkaConf(kc *conf.KafkaConfig) {
	s.kafkaConf = kc
}

func (s *NetworkPlugin) Start(test bool) (infos []*model.ListenerInfo, rerr error) {
	if s.cmd != nil {
		return nil, fmt.Errorf("Plugin already started")
	}
	exe, err := sys.Executable()
	if err != nil {
		return nil, err
	}

	args := []string{fmt.Sprintf("skewer-%s", s.t)}
	if test {
		args = append(args, "--test")
	}
	s.cmd = &exec.Cmd{
		Args:   args,
		Path:   exe,
		Stderr: os.Stderr,
		ExtraFiles: []*os.File{
			os.NewFile(uintptr(s.binderHandle), "binder"),
			os.NewFile(uintptr(s.loggerHandle), "logger"),
		},
		Env: []string{"PATH=/bin:/usr/bin"},
	}

	s.stdin, err = s.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	s.stdout, err = s.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = s.cmd.Start()
	if err != nil {
		return nil, err
	}

	infos = []*model.ListenerInfo{}
	startedChan := make(chan struct{})

	var once sync.Once

	go func() {
		// read JSON encoded messages that the plugin is going to
		// write on stdout
		var err error
		scanner := bufio.NewScanner(s.stdout)
		scanner.Split(PluginSplit)
		for scanner.Scan() {
			b := scanner.Bytes()
			if bytes.HasPrefix(b, []byte("syslog ")) {
				m := &model.TcpUdpParsedMessage{}
				err = json.Unmarshal(b[7:], m)
				if err == nil {
					s.stasher.Stash(m)
				} else {
					s.logger.Warn("Plugin sent a badly encoded JSON log line", "error", err)
				}
			} else if bytes.HasPrefix(b, []byte("started ")) {
				err := json.Unmarshal(b[8:], &infos)
				if err != nil {
					s.logger.Warn("Plugin sent a badly encoded JSON listener info", "error", err)
					rerr = err
				}
				once.Do(func() { close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("starterror ")) {
				rerr = fmt.Errorf(string(b[11:]))
				once.Do(func() { close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("syslogconferror ")) {
				rerr = fmt.Errorf(string(b[16:]))
				once.Do(func() { close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("parserconferror ")) {
				rerr = fmt.Errorf(string(b[16:]))
				once.Do(func() { close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("kafkaconferror ")) {
				rerr = fmt.Errorf(string(b[15:]))
				once.Do(func() { close(startedChan) })
			} else if bytes.HasPrefix(b, []byte("nolistenererror")) {
				rerr = nil
				infos = nil
				once.Do(func() { close(startedChan) })
				s.Stop()
				s.WaitClosed()
			} else {
				s.logger.Warn("Unexpected message from plugin", "message", string(b))
			}
		}
	}()

	scb, _ := json.Marshal(s.syslogConfs)
	pcb, _ := json.Marshal(s.parserConfs)
	kcb, _ := json.Marshal(s.kafkaConf)

	s.stdin.Write([]byte("syslogconf "))
	s.stdin.Write(scb)
	s.stdin.Write([]byte("\n"))

	s.stdin.Write([]byte("parserconf "))
	s.stdin.Write(pcb)
	s.stdin.Write([]byte("\n"))

	s.stdin.Write([]byte("kafkaconf "))
	s.stdin.Write(kcb)
	s.stdin.Write([]byte("\n"))

	s.stdin.Write([]byte("start\n"))
	<-startedChan

	if rerr != nil {
		infos = nil
		s.Stop()
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

func (p *NetworkPluginProvider) Launch(typ string, test bool, binderClient *sys.BinderClient, logger log15.Logger) {
	generator := utils.Generator(context.Background(), logger)
	p.logger = logger

	var scanner *bufio.Scanner
	var command string
	var args string
	scanner = bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		parts := strings.SplitN(strings.Trim(scanner.Text(), "\r\n "), " ", 2)
		command = parts[0]
		switch command {
		case "start":
			if p.syslogConfs == nil || p.parserConfs == nil || p.kafkaConf == nil {
				errs := fmt.Sprintf("syslogconferror syslog conf or parser conf was not provided to plugin")
				fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
				p.svc = nil
			} else {
				p.svc = NewNetworkService(typ, p, generator, binderClient, nil, logger)
				p.svc.SetConf(p.syslogConfs, p.parserConfs)
				p.svc.SetKafkaConf(p.kafkaConf)
				infos, err := p.svc.Start(test)
				if err != nil {
					errs := fmt.Sprintf("starterror %s", err.Error())
					fmt.Fprintf(os.Stdout, "%010d %s\n", len(errs), errs)
					p.svc = nil
				} else if len(infos) == 0 && typ != "skewer-relp" { // RELP never reports infos
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
		case "stop":
			if p.svc != nil {
				p.svc.Stop()
				p.svc.WaitClosed()
				p.svc = nil
			}
			os.Exit(0)
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
		}
	}
	e := scanner.Err()
	if e != nil {
		logger.Error("Scanning stdin error", "error", e)
		os.Exit(-1)
	}
	os.Exit(0)
}
