package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/awnumar/memguard"
	"github.com/gogo/protobuf/proto"
	"github.com/inconshreveable/log15"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/sys/namespaces"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/reservoir"
	"github.com/stephane-martin/skewer/utils/waiter"
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
var NOLISTENER = eerrors.New("no listener")

// Controller launches and controls the various services by distinct processes.
type Controller struct {
	typ  base.Types
	name string

	conf conf.BaseConfig

	pipe     *os.File
	logger   log15.Logger
	stasher  *StoreController
	registry *consul.Registry

	metricsChan chan []*dto.MetricFamily
	stdinMu     sync.Mutex
	stdinWriter *utils.SigWriter
	signKey     *memguard.LockedBuffer
	cmd         *namespaces.PluginCmd

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
	stasher  *StoreController
	registry *consul.Registry
	logger   log15.Logger
}

func ControllerFactory(ring kring.Ring, signKey *memguard.LockedBuffer, stasher *StoreController, registry *consul.Registry, logger log15.Logger) *CFactory {
	f := CFactory{
		ring:     ring,
		signKey:  signKey,
		stasher:  stasher,
		registry: registry,
		logger:   logger,
	}
	return &f
}

func (f *CFactory) New(typ base.Types) (*Controller, error) {
	name, err := base.Name(typ, false)
	if err != nil {
		return nil, err
	}
	s := Controller{
		typ:          typ,
		name:         name,
		stasher:      f.stasher,
		registry:     f.registry,
		logger:       f.logger,
		signKey:      f.signKey,
		ring:         f.ring,
		metricsChan:  make(chan []*dto.MetricFamily),
		ShutdownChan: make(chan struct{}),
	}
	return &s, nil
}

func (f *CFactory) NewStore(loggerHandle uintptr) *StoreController {
	st, _ := f.New(base.Store)
	return &StoreController{
		Controller: st,
		gen:        utils.NewGenerator(),
		reserv:     reservoir.NewReservoir(5000),
	}
}

// W encodes an writes a message to the controlled plugin via its stdin
func (s *Controller) W(header []byte, message []byte) (err error) {
	s.stdinMu.Lock()
	if s.stdinWriter != nil {
		err = eerrors.Wrap(s.stdinWriter.WriteWithHeader(header, message), "error writing to child stdin pipe")
	} else {
		err = eerrors.New("stdin is nil")
	}
	s.stdinMu.Unlock()
	return err
}

// Gather asks the controlled plugin to report its metrics
func (s *Controller) Gather() (m []*dto.MetricFamily, err error) {
	m = make([]*dto.MetricFamily, 0)
	select {
	case <-s.ShutdownChan:
		return nil, nil
	default:
		s.startedMu.Lock()
		started := s.started
		s.startedMu.Unlock()
		if !started {
			return nil, nil
		}
		if s.W(GATHER, utils.NOW) != nil {
			return nil, nil
		}

		select {
		case <-s.ShutdownChan:
			return nil, nil
		case <-time.After(2 * time.Second):
			s.logger.Debug("Child did not respond to metrics request after timeout", "type", s.typ)
			return nil, nil
		case metrics, more := <-s.metricsChan:
			if !more || metrics == nil {
				return
			}
			return metrics, nil
		}
	}
}

// Stop kindly asks the controlled plugin to stop activity
func (s *Controller) Stop() error {
	// in case the plugin was in fact never created...
	s.createdMu.Lock()
	created := s.created
	s.createdMu.Unlock()
	if !created {
		return nil
	}

	select {
	case <-s.ShutdownChan:
		return nil
	case <-s.StopChan:
		return nil
	default:
	}
	err := s.W(STOP, utils.NOW)
	if err != nil {
		return eerrors.Wrapf(err, "Error sending 'stop' message to plugin '%s'", s.name)
	}
	<-s.StopChan
	return nil
}

// Shutdown demands that the controlled plugin shutdowns now. After killTimeOut, it kills the plugin.
func (s *Controller) Shutdown(killTimeOut time.Duration) (killed bool) {
	// in case the plugin process was in fact never created...
	s.createdMu.Lock()
	created := s.created
	s.createdMu.Unlock()
	if !created {
		return false
	}

	select {
	case <-s.ShutdownChan:
		// the plugin process is already dead
		<-s.StopChan
		return false
	default:
		// ask to shutdown
		s.logger.Debug("Controller is asked to shutdown", "type", s.name)
		err := s.W(SHUTDOWN, utils.NOW)
		if err != nil {
			s.logger.Warn("Error writing shutdown to plugin stdin. Let's kill it brutally.", "error", err, "type", s.name)
			killTimeOut = time.Second
		}
		// wait for plugin process termination
		if killTimeOut == 0 {
			<-s.ShutdownChan
			<-s.StopChan
			return false
		}
		select {
		case <-s.ShutdownChan:
			<-s.StopChan
			return false
		case <-time.After(killTimeOut):
			// after timeout kill the process
			s.logger.Warn("Plugin failed to shutdown before timeout", "type", s.name)
			_ = s.kill(false)
			<-s.ShutdownChan
			<-s.StopChan
			return true
		}
	}

}

// SetConf gives the current global configuration to the controller.
// The controller will communicate the configuration to the controlled plugin at next start.
func (s *Controller) SetConf(c conf.BaseConfig) {
	s.conf = c
}

func (s *Controller) kill(misbevave bool) (err error) {
	if misbevave {
		s.logger.Crit("killing misbehaving plugin", "type", s.name)
	}
	s.stdinMu.Lock()
	err = s.cmd.Kill()
	s.stdinMu.Unlock()
	return err
}

type infosAndError struct {
	infos []model.ListenerInfo
	err   error
}

// listen for the encrypted messages that the plugin produces
func (s *Controller) listenpipe(secret *memguard.LockedBuffer) (err error) {
	if s.pipe == nil || s.typ == base.Store || s.typ == base.Configuration {
		return nil
	}
	scanner := utils.WithRecover(bufio.NewScanner(s.pipe))
	scanner.Split(utils.MakeDecryptSplit(secret))
	scanner.Buffer(make([]byte, 0, 132000), 132000)

	var message *model.FullMessage
	protobuff := proto.NewBuffer(make([]byte, 0, 4096))

	for scanner.Scan() {
		protobuff.SetBuf(scanner.Bytes())
		message, err = model.FromBuf(protobuff)
		if err != nil {
			return eerrors.Wrapf(err, "Unexpected error decrypting message from the plugin '%s' pipe", s.name)
		}
		err = s.stasher.Stash(message) // send message to the Store controller
		model.FullFree(message)
		if err != nil {
			return eerrors.Wrap(err, "Error stashing message")
		}

	}
	err = scanner.Err()
	if err == nil || eerrors.HasFileClosed(err) {
		s.logger.Debug("listenpipe stops", "type", s.name)
		return nil
	}
	if err != nil {
		return eerrors.Wrapf(err, "Unexpected error when listening to the plugin '%s' pipe", s.name)
	}
	return nil
}

func (s *Controller) listen(secret *memguard.LockedBuffer) chan infosAndError {
	startErrorChan := make(chan infosAndError)

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
			s.logger.Debug("Plugin controller is stopping", "type", s.name)
			startError(eerrors.New("unexpected end of plugin before it was initialized"), nil)

			s.createdMu.Lock()
			s.startedMu.Lock()
			s.started = false

			select {
			case <-s.ShutdownChan:
				// child process has already exited
				s.logger.Debug("Plugin child process has shut down", "type", s.name)
				s.created = false
			default:
				// child process is still alive, but we are in the defer(). why ?
				if kill {
					// the child misbehaved and deserved to be killed
					_ = s.kill(true)
					<-s.ShutdownChan
					s.created = false
				} else if normalStop {
					s.logger.Debug("Plugin child process has stopped normally", "type", s.name)
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
		scanner := utils.WithRecover(bufio.NewScanner(s.cmd.Stdout))
		scanner.Split(utils.PluginSplit)
		scanner.Buffer(make([]byte, 0, 132000), 132000)
		command := ""
		infos := make([]model.ListenerInfo, 0)

		for scanner.Scan() {
			parts := bytes.SplitN(scanner.Bytes(), space, 2)
			command = string(parts[0])
			switch command {
			case "syslog":
				// the plugin emitted a syslog message to be sent to the Store
				// in the general case the plugin should rather use the dedicated message pipe
				if len(parts) == 2 {
					if !initialized {
						err := eerrors.New("plugin sent a syslog message before being initialized")
						s.logger.Error(err.Error())
						startError(err, nil)
						kill = true
						return
					}
					m := model.FullFactory()
					err := m.Decrypt(secret, parts[1])
					if err == nil {
						err = s.stasher.Stash(m)
						model.FullFree(m)
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
					inf := make([]model.ListenerInfo, 0)
					err := json.Unmarshal([]byte(parts[1]), &inf)
					if err == nil {
						initialized = true
						startError(nil, inf)
					} else {
						err = eerrors.Wrap(err, "Plugin sent a badly encoded JSON listener info")
						s.logger.Warn(err.Error())
						startError(err, nil)
						kill = true
						return
					}
				}
			case "infos":
				if len(parts) == 2 {
					newinfos := make([]model.ListenerInfo, 0)
					err := json.Unmarshal([]byte(parts[1]), &newinfos)
					if err != nil {
						err = eerrors.Wrap(err, "Can't JSON decode listener info")
						s.logger.Warn(err.Error())
					} else {
						s.logger.Info("reported infos", "infos", newinfos, "type", s.name)
						if s.registry != nil {
							// register the listeners in consul
							for _, i := range infos {
								s.registry.UnregisterTcpListener(i.BindAddr, i.Protocol, i.Port)
							}
							for _, i := range newinfos {
								s.registry.RegisterTcpListener(i.BindAddr, i.Protocol, i.Port)
							}
							infos = newinfos
						}
					}
				}
			case "stopped":
				// plugin child says it has stopped, but the child process stays alive (useful for systemd child)
				normalStop = true
				return
			case "shutdown":
				// plugin child says it is shutting down, eventually the scanner will return normally, we just wait for that
			case "starterror":
				if len(parts) == 2 {
					err := fmt.Errorf(string(parts[1]))
					startError(err, nil)
				}
			case "conferror":
				if len(parts) == 2 {
					err := eerrors.Errorf("Plugin reports a configuration error: %s", string(parts[1]))
					s.logger.Warn(err.Error())
					startError(err, nil)
					// TODO: kill ?
				} else {
					err := eerrors.New("Plugin reports a configuration error, but the error is badly formatted")
					s.logger.Warn(err.Error())
					startError(err, nil)
					// TODO: kill ?
				}
			case "nolistenererror":
				startError(NOLISTENER, nil)
			case "metrics":
				if len(parts) == 2 {
					families := make([]*dto.MetricFamily, 0)
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
					s.logger.Error("Plugin returned badly formatted metrics")
					close(s.metricsChan)
					kill = true
					return
				}
			default:
				err := eerrors.New("unexpected message from plugin")
				s.logger.Error(err.Error(), "command", command)
				startError(err, nil)
				kill = true
				return
			}
		}
		err := scanner.Err()
		if err == nil {
			err = eerrors.Errorf("Plugin scanner returned: %s", s.name)
			s.logger.Debug(err.Error())
			startError(err, nil)
			// 'scanner' has returned without error.
			// so we know that the plugin child has exited
			// let's wait that the shutdown channel has been closed before executing the defer()
			<-s.ShutdownChan
		} else {
			if eerrors.HasFileClosed(err) {
				err = eerrors.Wrapf(err, "Plugin scanner returned: %s", s.name)
				startError(err, nil)
				<-s.ShutdownChan
			} else {
				// plugin has sent an invalid message that could not be interpreted by scanner
				err = eerrors.Wrapf(err, "Plugin scanner error: %s", s.name)
				startError(err, nil)
				s.logger.Warn(err.Error())
				kill = true
			}
		}
	}()
	return startErrorChan
}

// Start asks the controlled plugin to start the operations.
func (s *Controller) Start() (infos []model.ListenerInfo, err error) {
	s.createdMu.Lock()
	s.startedMu.Lock()
	if !s.created {
		s.startedMu.Unlock()
		s.createdMu.Unlock()
		return nil, eerrors.Errorf("can not start, plugin '%s' has not been created", s.name)
	}
	if s.started {
		s.startedMu.Unlock()
		s.createdMu.Unlock()
		return nil, eerrors.Errorf("plugin already started: '%s'", s.name)
	}
	s.StopChan = make(chan struct{})

	// setup the secret used to encrypt/decrypt messages
	var secret *memguard.LockedBuffer
	if s.conf.Main.EncryptIPC {
		s.logger.Debug("Decrypting messages from plugin", "type", s.name)
		secret, err = s.ring.GetBoxSecret()
		if err != nil {
			return nil, eerrors.Wrapf(err, "Can't get box secret")
		}
	}
	go func() {
		err := s.listenpipe(secret)
		if err != nil {
			s.logger.Error("listenpipe error", "err", err.Error(), "type", s.name)
		}
	}()

	// the s.conf is filtered so that only the needed parameters
	// are transmitted to the provider
	cb, _ := json.Marshal(Configure(s.typ, s.conf))

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
			rerr = eerrors.Errorf("plugin '%s' failed to start before timeout", s.name)
		}
	}

	if rerr != nil {
		s.startedMu.Unlock()
		s.createdMu.Unlock()
		if rerr != NOLISTENER {
			s.logger.Error("Start error", "error", rerr.Error(), "type", s.name)
		}
		s.Shutdown(3 * time.Second)
		return nil, rerr
	}

	s.started = true
	s.startedMu.Unlock()
	s.createdMu.Unlock()
	return infos, nil
}

type PluginCreateOpts struct {
	dumpable        bool
	profile         bool
	storePath       string
	confDir         string
	acctPath        string
	fileDestTmpl    string
	certFiles       []string
	certPaths       []string
	polldirectories []string
}

func ProfileOpt(profile bool) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.profile = profile
	}
}

func DumpableOpt(dumpable bool) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.dumpable = dumpable
	}
}

func StorePathOpt(path string) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.storePath = path
	}
}

func ConfDirOpt(path string) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.confDir = path
	}
}

func AccountingPathOpt(path string) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.acctPath = path
	}
}

func FileDestTmplOpt(path string) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.fileDestTmpl = path
	}
}

func CertFilesOpt(list []string) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.certFiles = list
	}
}

func CertPathsOpt(list []string) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.certPaths = list
	}
}

func PollDirectories(dirs []string) func(*PluginCreateOpts) {
	return func(opts *PluginCreateOpts) {
		opts.polldirectories = dirs
	}
}

func (s *Controller) Create(optsfuncs ...func(*PluginCreateOpts)) error {
	// if the provider process already lives, Create() just returns
	s.createdMu.Lock()
	if s.created {
		s.createdMu.Unlock()
		return nil
	}

	opts := &PluginCreateOpts{
		certFiles: make([]string, 0),
		certPaths: make([]string, 0),
	}
	for _, f := range optsfuncs {
		f(opts)
	}

	s.ShutdownChan = make(chan struct{})
	s.ExitCode = 0
	var err error

	switch s.typ {
	case base.RELP, base.TCP, base.UDP,
		base.DirectRELP,
		base.Graylog, base.KafkaSource, base.HTTPServer,
		base.Accounting, base.MacOS, base.Journal,
		base.Filesystem:

		cname, _ := base.Name(s.typ, true)
		// the plugin will use this pipe to report syslog messages
		piper, pipew, err := os.Pipe()
		if err != nil {
			close(s.ShutdownChan)
			s.createdMu.Unlock()
			return eerrors.Wrap(err, "Error creating plugin pipe")
		}
		s.pipe = piper

		// if creating the namespaces fails, fallback to classical start
		// this way we can support environments where user namespaces are not
		// available
		//noinspection GoBoolExpressions
		if capabilities.CapabilitiesSupported {
			s.cmd, err = namespaces.SetupCmd(
				cname,
				s.ring,
				namespaces.BinderHandle(base.BinderHdl(s.typ)),
				namespaces.LoggerHandle(base.LoggerHdl(s.typ)),
				namespaces.Pipe(pipew),
			)
			if err != nil {
				_ = piper.Close()
				_ = pipew.Close()
				close(s.ShutdownChan)
				s.createdMu.Unlock()
				return eerrors.Wrapf(err, "Error setting up the execution environment for plugin: %s", s.name)
			}
			err = s.cmd.Namespaced().
				Dumpable(opts.dumpable).
				AccountingPath(opts.acctPath).
				CertFiles(opts.certFiles).
				CertPaths(opts.certPaths).
				PollDirectories(opts.polldirectories).
				Start()
		}

		if err != nil {
			s.logger.Warn("Starting plugin in user namespace failed", "error", err, "type", s.name)
		}
		//noinspection GoBoolExpressions
		if err != nil || !capabilities.CapabilitiesSupported {
			s.cmd, err = namespaces.SetupCmd(
				s.name,
				s.ring,
				namespaces.BinderHandle(base.BinderHdl(s.typ)),
				namespaces.LoggerHandle(base.LoggerHdl(s.typ)),
				namespaces.Pipe(pipew),
			)
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

	case base.Store:
		cname, _ := base.Name(base.Store, true)
		piper, pipew, err := os.Pipe()
		if err != nil {
			close(s.ShutdownChan)
			s.createdMu.Unlock()
			return eerrors.Wrap(err, "Error creating plugin pipe")
		}
		s.pipe = pipew
		//noinspection GoBoolExpressions
		if capabilities.CapabilitiesSupported {
			s.cmd, err = namespaces.SetupCmd(
				cname,
				s.ring,
				namespaces.BinderHandle(base.BinderHdl(s.typ)),
				namespaces.LoggerHandle(base.LoggerHdl(s.typ)),
				namespaces.Pipe(piper),
				namespaces.Profile(opts.profile),
			)
			if err != nil {
				_ = piper.Close()
				_ = pipew.Close()
				close(s.ShutdownChan)
				s.createdMu.Unlock()
				return err
			}
			err = s.cmd.Namespaced().
				Dumpable(opts.dumpable).
				StorePath(opts.storePath).
				FileDestTemplate(opts.fileDestTmpl).
				CertFiles(opts.certFiles).
				CertPaths(opts.certPaths).
				Start()
		}

		if err != nil {
			s.logger.Warn("Starting plugin in user namespace failed", "error", err, "type", s.name)
		}
		//noinspection GoBoolExpressions
		if err != nil || !capabilities.CapabilitiesSupported {
			s.cmd, err = namespaces.SetupCmd(
				s.name,
				s.ring,
				namespaces.BinderHandle(base.BinderHdl(s.typ)),
				namespaces.LoggerHandle(base.LoggerHdl(s.typ)),
				namespaces.Pipe(piper),
				namespaces.Profile(opts.profile),
			)
			if err != nil {
				_ = piper.Close()
				_ = pipew.Close()
				close(s.ShutdownChan)
				s.createdMu.Unlock()
				return eerrors.Wrapf(err, "Error setting up the execution environment for plugin: %s", s.name)
			}
			err = s.cmd.Start()
		}
		_ = piper.Close()
		if err != nil {
			_ = pipew.Close()
		}

	default:
		s.cmd, err = namespaces.SetupCmd(
			s.name,
			s.ring,
			namespaces.BinderHandle(base.BinderHdl(s.typ)),
			namespaces.LoggerHandle(base.LoggerHdl(s.typ)),
		)
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
		return eerrors.Wrapf(err, "Plugin failed to start: %s", s.name)
	}
	s.stdinWriter = utils.NewSignatureWriter(s.cmd.Stdin, s.signKey)
	s.created = true
	s.createdMu.Unlock()

	go func() {
		// monitor plugin process termination
		err := s.cmd.Wait()
		if err == nil {
			s.logger.Debug("Plugin process has exited without reporting error", "type", s.name)
			s.ExitCode = 0
		} else {
			s.logger.Error("Plugin process has shutdown with error", "type", s.name, "error", err.Error())
			if e, ok := err.(*exec.ExitError); ok {
				status := e.ProcessState.Sys()
				s.ExitCode = 1
				if cstatus, ok := status.(syscall.WaitStatus); ok {
					s.ExitCode = cstatus.ExitStatus()
					s.logger.Error("Plugin process return code", "type", s.name, "code", s.ExitCode)
				}
			}
		}
		close(s.ShutdownChan)
		// after some client has waited ShutdownChan to be closed, it can safely read ExitCode
	}()
	return nil

}

// StoreController is the specialized controller that takes care of the Store.
type StoreController struct {
	*Controller
	reserv    *reservoir.Reservoir
	msgsBatch []string
	gen       *utils.Generator
	pushwg    sync.WaitGroup
}

func (s *StoreController) push(secret *memguard.LockedBuffer) {
	bufpipe := bufio.NewWriter(s.pipe)
	writeToStore := utils.NewEncryptWriter(bufpipe, secret)
	m := make(map[utils.MyULID]string, 5000)
	w := waiter.Default()

	for {
		err := s.reserv.DeliverTo(m)
		if err == eerrors.ErrQDisposed {
			return
		}

		if len(m) == 0 {
			w.Wait()
			continue
		}
		w.Reset()

		for _, v := range m {
			_, err := io.WriteString(writeToStore, v)
			if err != nil {
				s.logger.Error("Unexpected error when writing messages to the Store pipe", "error", err)
				return
			}
		}
		bufpipe.Flush()

		for k := range m {
			delete(m, k)
		}
	}
}

// Shutdown stops definetely the Store.
func (s *StoreController) Shutdown(killTimeOut time.Duration) {
	s.reserv.Dispose()                 // will make push() return
	s.pushwg.Wait()                    // wait that push() returns
	_ = s.pipe.Close()                 // signal the store that we are done sending messages
	s.Controller.Shutdown(killTimeOut) // shutdown the child
}

// Stash sends the given message to the Store
func (s *StoreController) Stash(m *model.FullMessage) error {
	if s.conf.Store.AddMissingMsgID && len(m.Fields.MsgId) == 0 {
		m.Fields.MsgId = m.Uid.String()
	}
	err := s.reserv.AddMessage(m)
	if err != nil {
		return eerrors.Wrap(err, "Failed to protobuf-marshal message to be sent to the Store")
	}
	return nil
}

func (s *StoreController) Start() (infos []model.ListenerInfo, err error) {
	var secret *memguard.LockedBuffer
	if s.conf.Main.EncryptIPC {
		secret, err = s.ring.GetBoxSecret()
		if err != nil {
			return nil, err
		}
	}

	infos, err = s.Controller.Start()
	if err != nil {
		return nil, err
	}
	s.pushwg.Add(1)
	go func() {
		defer s.pushwg.Done()
		s.push(secret)
	}()
	return infos, nil
}
