package macos

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

func initMacOSRegistry() {
	base.Once.Do(func() {
		base.InitRegistry()
	})
}

type MacLogsService struct {
	stasher        *base.Reporter
	logger         log15.Logger
	wgroup         sync.WaitGroup
	Conf           conf.MacOSSourceConfig
	stopchan       chan struct{}
	fatalErrorChan chan struct{}
	fatalOnce      *sync.Once
	confined       bool
	cmd            *exec.Cmd
	sync.Mutex
}

func NewMacOSLogsService(env *base.ProviderEnv) (base.Provider, error) {
	initMacOSRegistry()
	s := MacLogsService{
		stasher:  env.Reporter,
		logger:   env.Logger.New("class", "macos"),
		confined: env.Confined,
	}
	return &s, nil
}

func (s *MacLogsService) Type() base.Types {
	return base.MacOS
}

func (s *MacLogsService) Gather() ([]*dto.MetricFamily, error) {
	return base.Registry.Gather()
}

func (s *MacLogsService) FatalError() chan struct{} {
	return s.fatalErrorChan
}

func (s *MacLogsService) dofatal() {
	s.fatalOnce.Do(func() { close(s.fatalErrorChan) })
}

func (s *MacLogsService) Start() (infos []model.ListenerInfo, err error) {
	infos = []model.ListenerInfo{}
	s.Lock()
	defer s.Unlock()
	if s.cmd != nil {
		return infos, eerrors.New("already started")
	}

	s.stopchan = make(chan struct{})
	s.fatalErrorChan = make(chan struct{})
	s.fatalOnce = &sync.Once{}
	commandStr := s.Conf.Command
	args := []string{
		"stream",
		"--color=none",
		"--style=json",
	}
	level := "default"
	if len(s.Conf.Level) > 0 {
		level = s.Conf.Level
	}
	args = append(args, fmt.Sprintf("--level=%s", level))
	if len(s.Conf.Predicate) > 0 {
		args = append(args, fmt.Sprintf("--predicate=%s", s.Conf.Predicate))
	}
	if len(s.Conf.Process) > 0 {
		args = append(args, fmt.Sprintf("--process=%s", s.Conf.Process))
	}
	cmdObj := exec.Command(commandStr, args...)
	cmdObj.Stdin = nil
	stdout, err := cmdObj.StdoutPipe()
	if err != nil {
		return infos, err
	}
	stderr, err := cmdObj.StderrPipe()
	if err != nil {
		return infos, err
	}
	err = cmdObj.Start()
	if err != nil {
		return infos, err
	}
	s.logger.Info("log command has been started")
	s.cmd = cmdObj
	s.wgroup.Add(1)
	go s.logStderr(stderr)
	s.wgroup.Add(1)
	go s.parseStdout(stdout)

	return infos, nil
}

func (s *MacLogsService) logStderr(stderr io.ReadCloser) {
	defer s.wgroup.Done()
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		s.logger.Info(scanner.Text())
	}
}

func (s *MacLogsService) parseStdout(stdout io.ReadCloser) {
	defer s.wgroup.Done()
	dec := json.NewDecoder(stdout)
	dec.Token()
	var macoslog model.MacOSLogMessage
	hostname, _ := os.Hostname()
	var reported time.Time
	gen := utils.NewGenerator()

	for dec.More() {
		err := dec.Decode(&macoslog)
		if err != nil {
			s.logger.Error("Error parsing MacOS logs", "error", err)
			continue
		}
		// 2018-02-14 21:33:13.792980+0100
		// 2006-01-02 15:04:05.000000-0700
		full := model.FullFactory()
		full.Fields.Message = macoslog.EventMessage
		full.Fields.AppName = macoslog.ProcessImagePath
		full.Fields.Facility = model.Fuser
		full.Fields.HostName = hostname
		full.Fields.MsgId = ""
		full.Fields.ProcId = strconv.FormatUint(macoslog.ProcessID, 10)
		full.Fields.Severity = model.Sinfo
		full.Fields.TimeGeneratedNum = time.Now().UnixNano()
		reported, err = time.Parse("2006-01-02 15:04:05.000000-0700", macoslog.Timestamp)
		if err == nil {
			full.Fields.TimeReportedNum = reported.UnixNano()
		} else {
			fmt.Fprintln(os.Stderr, err)
			full.Fields.TimeReportedNum = full.Fields.TimeGeneratedNum
		}
		full.Fields.SetPriority()
		full.Fields.Structured = ""
		full.Fields.Version = 1
		if len(macoslog.Category) > 0 {
			full.Fields.SetProperty("macos", "category", macoslog.Category)
		}
		if len(macoslog.ProcessImageUUID) > 0 {
			full.Fields.SetProperty("macos", "processImageUUID", macoslog.ProcessImageUUID)
		}
		if len(macoslog.MessageType) > 0 {
			full.Fields.SetProperty("macos", "messageType", macoslog.MessageType)
		}
		if len(macoslog.TimezoneName) > 0 {
			full.Fields.SetProperty("macos", "timezoneName", macoslog.TimezoneName)
		}
		if len(macoslog.Subsystem) > 0 {
			full.Fields.SetProperty("macos", "subsystem", macoslog.Subsystem)
		}
		if len(macoslog.SenderImageUUID) > 0 {
			full.Fields.SetProperty("macos", "senderImageUUID", macoslog.SenderImageUUID)
		}
		if len(macoslog.SenderImagePath) > 0 {
			full.Fields.SetProperty("macos", "senderImagePath", macoslog.SenderImagePath)
		}
		full.Fields.SetProperty("macos", "processUniqueID", strconv.FormatUint(macoslog.ProcessUniqueID, 10))
		full.Fields.SetProperty("macos", "threadID", strconv.FormatUint(macoslog.ThreadID, 10))
		full.Fields.SetProperty("macos", "traceID", strconv.FormatUint(macoslog.TraceID, 10))
		full.Fields.SetProperty("macos", "activityID", strconv.FormatUint(macoslog.ActivityID, 10))
		full.Fields.SetProperty("macos", "machTimestamp", strconv.FormatUint(macoslog.MachTimestamp, 10))
		full.Fields.SetProperty("macos", "senderProgramCounter", strconv.FormatUint(macoslog.SenderProgramCounter, 10))
		full.ConfId = s.Conf.ConfID
		full.Uid = gen.Uid()
		err = s.stasher.Stash(full)
		if eerrors.Is("Fatal", err) {
			s.logger.Error("Fatal error stashing message", "error", err)
			s.dofatal()
			return
		}
		if err != nil {
			s.logger.Error("Error stashing message", "error", err)
			continue
		}
		base.IncomingMsgsCounter.WithLabelValues("macos", hostname, "", "").Inc()
	}

}

func (s *MacLogsService) Stop() {
	s.Lock()
	defer s.Unlock()
	if s.cmd == nil {
		return
	}
	s.cmd.Process.Kill()
	s.cmd.Wait()
	s.cmd = nil
	s.logger.Info("log command has been terminated")
	s.wgroup.Wait()
}

func (s *MacLogsService) Shutdown() {
	s.Stop()
}

func (s *MacLogsService) SetConf(c conf.BaseConfig) {
	s.Lock()
	s.Conf = c.MacOS
	s.Unlock()
}
