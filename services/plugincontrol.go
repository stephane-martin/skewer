package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/kardianos/osext"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/sys/namespaces"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/queue"
)

var space = []byte(" ")
var START = []byte("start")
var STARTED = []byte("started")
var STOP = []byte("stop")
var STOPPED = []byte("stopped")
var CONF = []byte("conf")
var CONFERROR = []byte("conferror")
var SHUTDOWN = []byte("shutdown")
var STARTERROR = []byte("starterror")
var GATHER = []byte("gathermetrics")
var METRICS = []byte("metrics")
var NOLISTENER = errors.New("")

// PluginController launches and controls the plugins services
type PluginController struct {
	typ Types

	conf conf.BaseConfig

	binderHandle uintptr
	loggerHandle uintptr
	pipe         *os.File
	logger       log15.Logger
	stasher      *StorePlugin
	registry     *consul.Registry

	metricsChan chan []*dto.MetricFamily
	stdin       io.WriteCloser
	stdinMu     sync.Mutex
	stdinWriter *utils.SigWriter
	signKey     *memguard.LockedBuffer
	stdout      io.ReadCloser
	cmd         *exec.Cmd

	ShutdownChan chan struct{}
	StopChan     chan struct{}
	// ExitCode should be read only after ShutdownChan has been closed
	ExitCode  int
	startedMu sync.Mutex
	createdMu sync.Mutex
	started   bool
	created   bool
	ring      kring.Ring
}

type CFactory struct {
	ring     kring.Ring
	signKey  *memguard.LockedBuffer
	stasher  *StorePlugin
	registry *consul.Registry
	logger   log15.Logger
}

func ControllerFactory(ring kring.Ring, signKey *memguard.LockedBuffer, stasher *StorePlugin, registry *consul.Registry, logger log15.Logger) *CFactory {
	f := CFactory{
		ring:     ring,
		signKey:  signKey,
		stasher:  stasher,
		registry: registry,
		logger:   logger,
	}
	return &f
}

func (f *CFactory) New(typ Types, binderHandle uintptr, loggerHandle uintptr) *PluginController {
	s := PluginController{
		typ:          typ,
		stasher:      f.stasher,
		registry:     f.registry,
		binderHandle: binderHandle,
		loggerHandle: loggerHandle,
		logger:       f.logger,
		signKey:      f.signKey,
		ring:         f.ring,
	}
	s.metricsChan = make(chan []*dto.MetricFamily)
	s.ShutdownChan = make(chan struct{})
	return &s
}

func (f *CFactory) NewStore(loggerHandle uintptr) *StorePlugin {
	s := &StorePlugin{PluginController: f.New(Store, 0, loggerHandle)}
	s.MessageQueue = queue.NewMessageQueue()
	s.pushwg = &sync.WaitGroup{}
	return s
}

// W encodes an writes a message to the controlled plugin via its stdin
func (s *PluginController) W(header []byte, message []byte) (err error) {
	s.stdinMu.Lock()
	if s.stdinWriter != nil {
		err = s.stdinWriter.WriteWithHeader(header, message)
	} else {
		err = fmt.Errorf("stdin is nil")
	}
	s.stdinMu.Unlock()
	return err
}

// Gather asks the controlled plugin to report its metrics
func (s *PluginController) Gather() (m []*dto.MetricFamily, err error) {
	m = []*dto.MetricFamily{}
	select {
	case <-s.ShutdownChan:
		return
	default:
		s.startedMu.Lock()
		defer s.startedMu.Unlock()
		if !s.started {
			return
		}
		if s.W(GATHER, utils.NOW) != nil {
			return
		}

		select {
		case <-time.After(2 * time.Second):
			return
		case metrics, more := <-s.metricsChan:
			if !more || metrics == nil {
				return
			}
			return metrics, nil
		}
	}
}

// Stop kindly asks the controlled plugin to stop activity
func (s *PluginController) Stop() {
	// in case the plugin was in fact never created...
	s.createdMu.Lock()
	if !s.created {
		s.createdMu.Unlock()
		return
	}
	s.createdMu.Unlock()

	select {
	case <-s.ShutdownChan:
	case <-s.StopChan:
	default:
		err := s.W(STOP, utils.NOW)
		if err != nil {
			s.logger.Warn("Error writing stop to plugin stdin", "error", err)
		} else {
			<-s.StopChan
		}
	}
}

// Shutdown demands that the controlled plugin shutdowns now. After killTimeOut, it kills the plugin.
func (s *PluginController) Shutdown(killTimeOut time.Duration) {
	// in case the plugin process was in fact never created...
	s.createdMu.Lock()
	if !s.created {
		s.createdMu.Unlock()
		return
	}
	s.createdMu.Unlock()
	name := Types2Names[s.typ]

	select {
	case <-s.ShutdownChan:
		// the plugin process is already dead
		<-s.StopChan
	default:
		// ask to shutdown
		s.logger.Debug("Controller is asked to shutdown", "type", name)
		err := s.W(SHUTDOWN, utils.NOW)
		if err != nil {
			s.logger.Warn("Error writing shutdown to plugin stdin. Kill brutally.", "error", err, "type", name)
			killTimeOut = time.Second
		}
		// wait for plugin process termination
		if killTimeOut == 0 {
			<-s.ShutdownChan
			<-s.StopChan
		} else {
			select {
			case <-s.ShutdownChan:
				<-s.StopChan
			case <-time.After(killTimeOut):
				// after timeout kill the process
				s.logger.Warn("Plugin failed to shutdown before timeout", "type", name)
				_ = s.kill(false)
				<-s.ShutdownChan
				<-s.StopChan
			}
		}
	}

}

// SetConf gives the current global configuration to the controller.
// The controller will communicate the configuration to the controlled plugin at next start.
func (s *PluginController) SetConf(c conf.BaseConfig) {
	s.conf = c
}

func (s *PluginController) kill(misbevave bool) (err error) {
	if misbevave {
		s.logger.Crit("killing misbehaving plugin", "type", Types2Names[s.typ])
	}
	s.stdinMu.Lock()
	err = s.cmd.Process.Kill()
	s.stdinMu.Unlock()
	return err
}

type infosAndError struct {
	infos []model.ListenerInfo
	err   error
}

// listen for the encrypted messages that the plugin produces
func (s *PluginController) listenpipe(secret *memguard.LockedBuffer) {
	if s.pipe == nil {
		return
	}
	switch s.typ {
	case RELP, TCP, UDP, Accounting, Journal, KafkaSource:
	default:
		return
	}
	name := Types2Names[s.typ]
	scanner := bufio.NewScanner(s.pipe)
	scanner.Split(utils.MakeDecryptSplit(secret))
	scanner.Buffer(make([]byte, 0, 132000), 132000)

	var err error
	var buf []byte
	var message model.FullMessage

	for scanner.Scan() {
		buf = scanner.Bytes()
		_, err = message.UnmarshalMsg(buf)
		if err != nil {
			s.logger.Error("Unexpected error decrypting message from the plugin pipe", "type", name, "error", err)
			return
		}
		err = s.stasher.Stash(message)
		if err != nil {
			s.logger.Error("Error stashing message", "error", err)
			return
		}

	}
	err = scanner.Err()
	if err == io.EOF || err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF {
		s.logger.Debug("listenpipe stops", "type", s.typ)
	} else if err != nil {
		s.logger.Error("Unexpected error when listening to the plugin pipe", "type", name, "error", err)
	}
}

func (s *PluginController) listen(secret *memguard.LockedBuffer) chan infosAndError {
	startErrorChan := make(chan infosAndError)
	name := Types2Names[s.typ]

	var once sync.Once
	startError := func(err error, infos []model.ListenerInfo) {
		once.Do(func() {
			startErrorChan <- infosAndError{
				err:   err,
				infos: infos,
			}
			close(startErrorChan)
		})
	}

	go func() {
		initialized := false
		kill := false
		normalStop := false

		defer func() {
			s.logger.Debug("Plugin controller is stopping", "type", name)
			startError(fmt.Errorf("Unexpected end of plugin before it was initialized"), nil)

			s.createdMu.Lock()
			s.startedMu.Lock()
			s.started = false

			select {
			case <-s.ShutdownChan:
				// child process has already exited
				s.logger.Debug("Plugin child process has shut down", "type", name)
				s.created = false
			default:
				// child process is still alive, but we are in the defer(). why ?
				if kill {
					// the child misbehaved and deserved to be killed
					_ = s.kill(true)
					<-s.ShutdownChan
					s.created = false
				} else if normalStop {
					s.logger.Debug("Plugin child process has stopped normally", "type", name)
				} else {
					// should not happen, we assume an anomaly
					_ = s.kill(true)
					<-s.ShutdownChan
					s.created = false
				}
			}

			s.startedMu.Unlock()
			s.createdMu.Unlock()
			close(s.StopChan)
		}() // end of defer

		// read the encoded messages that the plugin may write on stdout
		scanner := bufio.NewScanner(s.stdout)
		scanner.Split(utils.PluginSplit)
		scanner.Buffer(make([]byte, 0, 132000), 132000)
		command := ""
		infos := []model.ListenerInfo{}
		var m model.FullMessage

		for scanner.Scan() {
			parts := bytes.SplitN(scanner.Bytes(), space, 2)
			command = string(parts[0])
			switch command {
			case "syslog":
				// the plugin emitted a syslog message to be sent to the Store
				if len(parts) == 2 {
					if !initialized {
						msg := "Plugin sent a syslog message before being initialized"
						s.logger.Error(msg)
						startError(fmt.Errorf(msg), nil)
						kill = true
						return
					}
					m = model.FullMessage{}
					err := m.Decrypt(secret, parts[1])
					if err == nil {
						err = s.stasher.Stash(m)
						if err != nil {
							s.logger.Error("Error stashing message", "error", err)
							kill = true
							return
						}
					} else {
						s.logger.Warn("Plugin sent a badly encoded log line", "error", err)
						kill = true
						return
					}
				}
			case "started":
				if len(parts) == 2 {
					// fill infos about listening ports
					infos := []model.ListenerInfo{}
					err := json.Unmarshal([]byte(parts[1]), &infos)
					if err == nil {
						initialized = true
						startError(nil, infos)
					} else {
						s.logger.Warn("Plugin sent a badly encoded JSON listener info", "error", err)
						startError(err, nil)
						kill = true
						return
					}
				}
			case "infos":
				if len(parts) == 2 {
					newinfos := []model.ListenerInfo{}
					err := json.Unmarshal([]byte(parts[1]), &newinfos)
					if err == nil {
						s.logger.Info("reported infos", "infos", newinfos, "type", name)
						if s.registry != nil {
							s.registry.UnregisterTcpListeners(infos)
							s.registry.RegisterTcpListeners(newinfos)
							infos = newinfos
						}
					}
				}
			case "stopped":
				normalStop = true
				return
			case "shutdown":
				// plugin child is shutting down, eventually the scanner will return normally, we just wait for that
			case "starterror":
				if len(parts) == 2 {
					err := fmt.Errorf(string(parts[1]))
					startError(err, nil)
				}
			case "conferror":
				if len(parts) == 2 {
					err := fmt.Errorf(string(parts[1]))
					startError(err, nil)
				}
			case "nolistenererror":
				startError(NOLISTENER, nil)
			case "metrics":
				if len(parts) == 2 {
					families := []*dto.MetricFamily{}
					err := json.Unmarshal([]byte(parts[1]), &families)
					if err == nil {
						s.metricsChan <- families
					} else {
						s.logger.Error("Plugin returned invalid metrics")
						close(s.metricsChan)
						kill = true
						return
					}
				} else {
					s.logger.Error("Plugin returned empty metrics")
					close(s.metricsChan)
					kill = true
					return
				}
			default:
				err := fmt.Errorf("Unexpected message from plugin")
				s.logger.Error("Unexpected message from plugin", "command", command)
				startError(err, nil)
				kill = true
				return
			}
		}
		err := scanner.Err()
		s.logger.Debug("Plugin scanner returned", "type", name)
		if err == nil {
			// 'scanner' has returned without error.
			// It means that the plugin child stdout is EOF = closed.
			// So we know that the plugin child has exited
			// Let's wait that the shutdown channel has been closed before executing the defer()
			<-s.ShutdownChan
		} else {
			// plugin has sent an invalid message that could not be interpreted by scanner
			startError(err, nil)
			if err == io.EOF || err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF || err == os.ErrClosed {
				<-s.ShutdownChan
			} else {
				s.logger.Warn("Plugin scanner error", "type", name, "error", err)
				kill = true
			}
		}
	}()
	return startErrorChan
}

// Start asks the controlled plugin to start the operations.
func (s *PluginController) Start() (infos []model.ListenerInfo, err error) {
	name := Types2Names[s.typ]
	s.createdMu.Lock()
	s.startedMu.Lock()
	if !s.created {
		s.startedMu.Unlock()
		s.createdMu.Unlock()
		return nil, fmt.Errorf("Can not start, plugin '%s' has not been created", name)
	}
	if s.started {
		s.startedMu.Unlock()
		s.createdMu.Unlock()
		return nil, fmt.Errorf("Plugin already started: %s", name)
	}
	s.StopChan = make(chan struct{})

	// setup the secret used to encrypt/decrypt messages
	var secret *memguard.LockedBuffer
	if s.conf.Main.EncryptIPC {
		s.logger.Debug("Decrypting messages from plugin", "type", name)
		secret, err = s.ring.GetBoxSecret()
		if err != nil {
			return nil, err
		}
	}
	go s.listenpipe(secret)

	cb, _ := json.Marshal(s.conf)

	rerr := s.W(CONF, cb)
	if rerr == nil {
		rerr = s.W(START, utils.NOW)
	}
	infos = []model.ListenerInfo{}
	if rerr == nil {
		select {
		case infoserr := <-s.listen(secret):
			rerr = infoserr.err
			infos = infoserr.infos
		case <-time.After(60 * time.Second):
			close(s.StopChan)
			rerr = fmt.Errorf("Plugin '%s' failed to start before timeout", name)
		}
	}

	if rerr != nil {
		s.startedMu.Unlock()
		s.createdMu.Unlock()
		if rerr != NOLISTENER {
			s.logger.Error("Start error", "error", rerr.Error(), "type", name)
		}
		s.Shutdown(3 * time.Second)
		return nil, rerr
	}

	s.started = true
	s.startedMu.Unlock()
	s.createdMu.Unlock()
	return infos, nil
}

func setupCmd(name string, r kring.Ring, binderHandle, loggerHandle uintptr, messagePipe *os.File, test bool) (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
	exe, err := osext.Executable()
	if err != nil {
		return nil, nil, nil, err
	}
	envs := []string{"PATH=/bin:/usr/bin", fmt.Sprintf("SKEWER_SESSION=%s", r.GetSessionID().String())}
	files := []*os.File{}
	if binderHandle != 0 {
		files = append(files, os.NewFile(uintptr(binderHandle), "binder"))
		envs = append(envs, "SKEWER_HAS_BINDER=TRUE")
	}
	if loggerHandle != 0 {
		files = append(files, os.NewFile(uintptr(loggerHandle), "logger"))
		envs = append(envs, "SKEWER_HAS_LOGGER=TRUE")
	}
	if messagePipe != nil {
		files = append(files, messagePipe)
		envs = append(envs, "SKEWER_HAS_PIPE=TRUE")
	}
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, err
	}
	files = append(files, rPipe)
	err = r.WriteRingPass(wPipe)
	_ = wPipe.Close()
	if err != nil {
		return nil, nil, nil, err
	}
	if test {
		envs = append(envs, "SKEWER_TEST=TRUE")
	}

	cmd := &exec.Cmd{
		Path:       exe,
		Stderr:     os.Stderr,
		ExtraFiles: files,
		Env:        envs,
		Args:       []string{name},
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	return cmd, in, out, nil
}

func (s *PluginController) Create(test bool, dumpable bool, storePath, confDir, acctPath, fileDestTmpl string) error {
	// if the provider process already lives, Create() just returns
	s.createdMu.Lock()
	if s.created {
		s.createdMu.Unlock()
		return nil
	}

	s.ShutdownChan = make(chan struct{})
	s.ExitCode = 0
	var err error
	name := Types2Names[s.typ]
	if s.typ != Accounting {
		acctPath = ""
	}
	if s.typ != Store {
		fileDestTmpl = ""
	}

	switch s.typ {
	case RELP, TCP, UDP, Accounting, Journal, KafkaSource:
		// the plugin will use this pipe to report syslog messages
		piper, pipew, err := os.Pipe()
		if err != nil {
			close(s.ShutdownChan)
			s.createdMu.Unlock()
			return err
		}
		s.pipe = piper

		// if creating the namespaces fails, fallback to classical start
		// this way we can support environments where user namespaces are not
		// available
		if capabilities.CapabilitiesSupported {
			s.cmd, s.stdin, s.stdout, err = setupCmd(fmt.Sprintf("confined-%s", name), s.ring, s.binderHandle, s.loggerHandle, pipew, test)
			if err != nil {
				_ = piper.Close()
				_ = pipew.Close()
				close(s.ShutdownChan)
				s.createdMu.Unlock()
				return err
			}
			err = namespaces.StartInNamespaces(s.cmd, dumpable, "", "", acctPath, "")
		}

		if err != nil {
			s.logger.Warn("Starting plugin in user namespace failed", "error", err, "type", name)
		}
		if err != nil || !capabilities.CapabilitiesSupported {
			s.cmd, s.stdin, s.stdout, err = setupCmd(name, s.ring, s.binderHandle, s.loggerHandle, pipew, test)
			if err != nil {
				_ = piper.Close()
				_ = pipew.Close()
				close(s.ShutdownChan)
				s.createdMu.Unlock()
				return err
			}
			err = s.cmd.Start()
		}
		_ = pipew.Close()
		if err != nil {
			_ = piper.Close()
		}

	case Store:
		piper, pipew, err := os.Pipe()
		if err != nil {
			close(s.ShutdownChan)
			s.createdMu.Unlock()
			return err
		}
		s.pipe = pipew
		if capabilities.CapabilitiesSupported {
			s.cmd, s.stdin, s.stdout, err = setupCmd(fmt.Sprintf("confined-%s", name), s.ring, s.binderHandle, s.loggerHandle, piper, test)
			if err != nil {
				_ = piper.Close()
				_ = pipew.Close()
				close(s.ShutdownChan)
				s.createdMu.Unlock()
				return err
			}
			err = namespaces.StartInNamespaces(s.cmd, dumpable, storePath, "", "", fileDestTmpl)
		}

		if err != nil {
			s.logger.Warn("Starting plugin in user namespace failed", "error", err, "type", name)
		}
		if err != nil || !capabilities.CapabilitiesSupported {
			s.cmd, s.stdin, s.stdout, err = setupCmd(name, s.ring, s.binderHandle, s.loggerHandle, piper, test)
			if err != nil {
				_ = piper.Close()
				_ = pipew.Close()
				close(s.ShutdownChan)
				s.createdMu.Unlock()
				return err
			}
			err = s.cmd.Start()
		}
		_ = piper.Close()
		if err != nil {
			_ = pipew.Close()
		}

	default:
		s.cmd, s.stdin, s.stdout, err = setupCmd(name, s.ring, s.binderHandle, s.loggerHandle, nil, test)
		if err != nil {
			close(s.ShutdownChan)
			s.createdMu.Unlock()
			return err
		}
		err = s.cmd.Start()
	}

	if err != nil {
		close(s.ShutdownChan)
		s.createdMu.Unlock()
		return err
	}
	s.stdinWriter = utils.NewSignatureWriter(s.stdin, s.signKey)
	s.created = true
	s.createdMu.Unlock()

	go func() {
		// monitor plugin process termination
		err := s.cmd.Wait()
		if err == nil {
			s.logger.Debug("Plugin process has exited without reporting error", "type", name)
			s.ExitCode = 0
		} else if e, ok := err.(*exec.ExitError); ok {
			s.logger.Error("Plugin process has shutdown with error", "stderr", string(e.Stderr), "type", name, "error", e.Error())
			status := e.ProcessState.Sys()
			if cstatus, ok := status.(syscall.WaitStatus); ok {
				s.ExitCode = cstatus.ExitStatus()
				s.logger.Error("Plugin process return code", "type", name, "code", s.ExitCode)
			} else {
				s.ExitCode = -1
				s.logger.Warn("Could not interpret plugin process return code", "type", name)
			}
		} else {
			s.logger.Error("Plugin process has exited abnormally, but the error could not be interpreted", "type", name, "error", err.Error())
		}
		close(s.ShutdownChan)
		// after some client has waited ShutdownChan to be closed, it can safely read ExitCode
	}()
	return nil

}

// StorePlugin is the specialized controller that takes care of the Store.
type StorePlugin struct {
	*PluginController
	*queue.MessageQueue
	pushwg *sync.WaitGroup
}

func (s *StorePlugin) pushqueue(secret *memguard.LockedBuffer) {
	var messages []*model.FullMessage
	var message *model.FullMessage
	var messageb []byte
	var err error
	bufpipe := bufio.NewWriter(s.pipe)
	writeToStore := utils.NewEncryptWriter(bufpipe, secret)

	for {
		messages = s.MessageQueue.GetMany(1000)
		if len(messages) == 0 {
			return
		}
		for _, message = range messages {
			messageb, err = message.MarshalMsg(nil)
			if err == nil {
				_, err = writeToStore.Write(messageb)
				if err != nil {
					s.logger.Error("Unexpected error when writing messages to the Store pipe", "error", err)
					return
				}
			} else {
				s.logger.Warn("A message provided by a plugin could not be serialized", "error", err)
			}
		}
		err = bufpipe.Flush()
		if err != nil {
			s.logger.Error("Unexpected error when flushing the Store pipe", "error", err)
			return
		}
	}
}

func (s *StorePlugin) push(secret *memguard.LockedBuffer) {
	for s.MessageQueue.Wait(0) {
		s.pushqueue(secret)
	}
	s.pushwg.Done()
}

// Shutdown stops definetely the Store.
func (s *StorePlugin) Shutdown(killTimeOut time.Duration) {
	s.MessageQueue.Dispose() // will make push() return
	s.pushwg.Wait()          // wait that push() returns

	if s.conf.Main.EncryptIPC {
		secret, err := s.ring.GetBoxSecret()
		if err == nil {
			s.pushqueue(secret) // empty the queue, in case there are pending messages
		}
	}

	_ = s.pipe.Close()                       // signal the store that we are done sending messages
	s.PluginController.Shutdown(killTimeOut) // shutdown the child
}

// Stash stores the given message into the Store
func (s *StorePlugin) Stash(m model.FullMessage) error {
	// this method is called very frequently, so we avoid to lock anything
	// the MessageQueue ensures that we write the messages sequentially to the store child
	return s.MessageQueue.Put(m)
}

func (s *StorePlugin) Start() (infos []model.ListenerInfo, err error) {
	var secret *memguard.LockedBuffer
	if s.conf.Main.EncryptIPC {
		secret, err = s.ring.GetBoxSecret()
		if err != nil {
			return nil, err
		}
	}

	infos, err = s.PluginController.Start()
	if err != nil {
		return nil, err
	}
	s.pushwg.Add(1)
	go s.push(secret)
	return infos, nil
}
