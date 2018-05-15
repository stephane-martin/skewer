package cmd

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/services/macos"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/logging"
)

var serveCobraCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start listening for Syslog messages and forward them to Kafka",
	Long: `The serve command is the main skewer command. It launches a long
running process that listens to syslog messages according to the configuration,
connects to Kafka, and forwards messages to Kafka.`,
	Run: func(cmd *cobra.Command, args []string) {},
}

var SyslogFlag bool
var LoglevelFlag string
var LogfilenameFlag string
var LogjsonFlag bool
var pidFilenameFlag string
var consulRegisterFlag bool
var consulServiceName string
var UidFlag string
var GidFlag string
var DumpableFlag bool
var profile bool

func init() {
	RootCmd.AddCommand(serveCobraCmd)
	serveCobraCmd.Flags().BoolVar(&SyslogFlag, "syslog", false, "Send logs to the local syslog (are you sure you wan't to do that ?)")
	serveCobraCmd.Flags().StringVar(&LoglevelFlag, "loglevel", "info", "Set logging level")
	serveCobraCmd.Flags().StringVar(&LogfilenameFlag, "logfilename", "", "Write logs to a file instead of stderr")
	serveCobraCmd.Flags().BoolVar(&LogjsonFlag, "logjson", false, "Write logs in JSON format")
	serveCobraCmd.Flags().StringVar(&pidFilenameFlag, "pidfile", "", "If given, write PID to file")
	serveCobraCmd.Flags().BoolVar(&consulRegisterFlag, "register", false, "Register services in consul")
	serveCobraCmd.Flags().StringVar(&consulServiceName, "servicename", "skewer", "Service name to register in consul")
	serveCobraCmd.Flags().StringVar(&UidFlag, "uid", "", "Switch to this user ID (when launched as root)")
	serveCobraCmd.Flags().StringVar(&GidFlag, "gid", "", "Switch to this group ID (when launched as root)")
	serveCobraCmd.Flags().BoolVar(&DumpableFlag, "dumpable", false, "if set, the skewer process will be traceable/dumpable")
	serveCobraCmd.Flags().BoolVar(&profile, "prof", false, "if set, profile memory")
}

// ExecuteChild sets up the environment for the serve command and starts it.
func ExecuteChild() (err error) {
	sessionID := strings.TrimSpace(os.Getenv("SKEWER_SESSION"))
	if len(sessionID) == 0 {
		return eerrors.New("empty session ID")
	}
	ringSecretPipe := os.NewFile(uintptr(len(base.Handles)+3), "ringsecretpipe")
	var ringSecret *memguard.LockedBuffer
	buf := make([]byte, 32)
	_, err = ringSecretPipe.Read(buf)
	if err != nil {
		return eerrors.Wrap(err, "Error reading ring secret")
	}
	ringSecret, err = memguard.NewImmutableFromBytes(buf)
	if err != nil {
		return eerrors.Wrap(err, "Error making memory guard")
	}
	creds := kring.RingCreds{Secret: ringSecret, SessionID: utils.MustParseULID(sessionID)}
	ring := kring.GetRing(creds)
	secret, err := ring.GetBoxSecret()
	if err != nil {
		return eerrors.Wrap(err, "Error creating box secret")
	}
	ch, err := newServeChild(ring)
	if err != nil {
		return eerrors.Wrap(err, "fatal error initializing main child")
	}
	err = ch.init()
	if err != nil {
		return eerrors.Wrap(err, "fatal error initializing Serve()")
	}
	defer func() {
		e := ch.cleanup()
		if e != nil {
			ch.logger.Warn("Serve() cleanup error", "error", e.Error())
		}
		secret.Destroy()
	}()
	err = ch.Serve()
	if err != nil {
		return eerrors.Wrap(err, "Fatal error executing Serve()")
	}
	return nil
}

type serveChild struct {
	globalCtx      context.Context
	globalCancel   context.CancelFunc
	shutdownCtx    context.Context
	shutdown       context.CancelFunc
	loggerCtx      context.Context
	cancelLogger   context.CancelFunc
	logger         log15.Logger
	confService    *services.ConfigurationService
	confChan       chan *conf.BaseConfig
	conf           *conf.BaseConfig
	consulParams   consul.ConnParams
	consulRegistry *consul.Registry
	store          *services.StorePlugin
	controllers    map[base.Types]*services.PluginController
	metricsServer  *metrics.MetricsServer
	signPrivKey    *memguard.LockedBuffer
	ring           kring.Ring
}

func newServeChild(ring kring.Ring) (*serveChild, error) {
	secret, err := ring.GetBoxSecret()
	if err != nil {
		return nil, eerrors.Wrap(err, "Error getting box secret")
	}
	childLoggerHdl := base.HandlesMap[base.ServiceHandle{Service: "child", Type: base.Logger}]
	conn, err := net.FileConn(os.NewFile(childLoggerHdl, "logger"))
	if err != nil {
		return nil, eerrors.Wrap(err, "Can't make connection to parent logger")
	}
	c := serveChild{ring: ring}
	c.globalCtx, c.globalCancel = context.WithCancel(context.Background())
	c.shutdownCtx, c.shutdown = context.WithCancel(c.globalCtx)

	loggerConn := conn.(*net.UnixConn)
	_ = loggerConn.SetReadBuffer(65536)
	_ = loggerConn.SetWriteBuffer(65536)
	c.loggerCtx, c.cancelLogger = context.WithCancel(context.Background())
	c.logger = logging.NewRemoteLogger(c.loggerCtx, loggerConn, secret).New("proc", "child")
	return &c, nil
}

func (ch *serveChild) init() error {
	err := ch.setupConsulRegistry()
	if err != nil {
		return eerrors.Wrap(err, "Error setting up consul registry")
	}

	err = ch.setupSignKey()
	if err != nil {
		return eerrors.Wrap(err, "Error setting up signature key")
	}

	err = ch.setupConfiguration()
	if err != nil {
		return eerrors.Wrap(err, "Error setting up configuration service")
	}

	st, err := ch.setupStore()
	if err != nil {
		return eerrors.Wrap(err, "Error setting up the store")
	}
	ch.store = st

	ch.setupControllers()
	ch.setupMetrics(ch.logger)
	return nil
}

func (ch *serveChild) cleanup() (err error) {
	errs := ch.ShutdownControllers()
	if !errs.Empty() {
		err = errs.Wrap("Error shutting down controllers")
	}
	ch.cancelLogger()
	ch.shutdown()
	ch.globalCancel()
	if ch.signPrivKey != nil {
		ch.signPrivKey.Destroy()
	}
	_ = ch.ring.Destroy()
	return err
}

// ShutdownControllers definitely shutdowns the plugin processes.
func (ch *serveChild) ShutdownControllers() eerrors.ErrorSlice {
	funcs := make([]utils.Func, 0, len(base.Names2Types))
	for n, t := range base.Names2Types {
		typ := t
		name := n
		switch typ {
		case base.Store, base.Configuration:
			// shutdown them later
		default:
			funcs = append(funcs, func() error {
				return eerrors.Wrapf(ch.StopController(typ, true), "Error shutting down controller '%s'", name)
			})
		}
	}
	errs := utils.Parallel(funcs...)

	/*
		ch.logger.Debug("The RELP service has been stopped")
		ch.logger.Debug("Stopped accounting service")
		ch.logger.Debug("Stopped journald service")
		ch.logger.Debug("The TCP service has been stopped")
		ch.logger.Debug("The UDP service has been stopped")
	*/

	ch.globalCancel()
	ch.store.Shutdown(150 * time.Second)
	if ch.consulRegistry != nil {
		ch.consulRegistry.WaitFinished() // wait that the services have been unregistered from Consul
	}
	ch.confService.Stop()
	return errs
}

func (ch *serveChild) setupConfiguration() error {
	confService, err := services.NewConfigurationService(ch.ring, ch.signPrivKey, base.LoggerHdl(base.Configuration), ch.logger)
	if err != nil {
		return eerrors.Wrap(err, "Error creating configuration service")
	}
	ch.confService = confService
	ch.confService.SetConfDir(configDirName)
	ch.confService.SetConsulParams(ch.consulParams)
	err = ch.confService.Start(ch.ring)
	if err != nil {
		return eerrors.Wrap(err, "error starting the configuration service")
	}
	ch.confChan = ch.confService.Chan()
	if ch.confChan == nil {
		return eerrors.New("Error starting the configuration service: conf channel is nil")
	}
	ch.conf = <-ch.confChan
	ch.conf.Store.Dirname = storeDirname
	ch.logger.Info("Store location", "path", ch.conf.Store.Dirname)
	return nil
}

func (ch *serveChild) setupConsulRegistry() error {
	ch.consulParams = consul.ConnParams{
		Address:    consulAddr,
		Datacenter: consulDC,
		Token:      consulToken,
		CAFile:     consulCAFile,
		CAPath:     consulCAPath,
		CertFile:   consulCertFile,
		KeyFile:    consulKeyFile,
		Insecure:   consulInsecure,
		Key:        consulPrefix,
	}
	var err error
	if consulRegisterFlag {
		ch.consulRegistry, err = consul.NewRegistry(ch.globalCtx, ch.consulParams, consulServiceName, ch.logger)
		if err != nil {
			return eerrors.Wrap(err, "Error building consul registry")
		}
	}
	return nil
}

func (ch *serveChild) setupStore() (st *services.StorePlugin, err error) {
	f := services.ControllerFactory(ch.ring, ch.signPrivKey, nil, ch.consulRegistry, ch.logger)
	st = f.NewStore(base.LoggerHdl(base.Store))
	st.SetConf(*ch.conf)

	tmpl := ""
	dests, _ := ch.conf.Main.GetDestinations()
	if (dests & conf.File) != 0 {
		tmpl = ch.conf.FileDest.Filename
	}

	certfiles := ch.conf.GetCertificateFiles()["dests"]
	certpaths := ch.conf.GetCertificatePaths()["dests"]

	err = st.Create(
		services.DumpableOpt(DumpableFlag),
		services.StorePathOpt(storeDirname),
		services.FileDestTmplOpt(tmpl),
		services.CertFilesOpt(certfiles),
		services.CertPathsOpt(certpaths),
		services.ProfileOpt(profile),
	)
	if err != nil {
		return nil, eerrors.Wrap(err, "can't create the store")
	}

	_, err = st.Start()
	if err != nil {
		return nil, eerrors.Wrap(err, "can't start the forwarders")
	}
	return st, nil
}

func (ch *serveChild) setupSignKey() error {
	ch.logger.Debug("Generating signature keys")
	privkey, err := ch.ring.NewSignaturePubkey()
	if err != nil {
		return eerrors.Wrap(err, "Error generating signature keys")
	}
	ch.signPrivKey = privkey
	return nil
}

func setupController(f *services.CFactory, typ base.Types) *services.PluginController {
	switch typ {
	case base.Configuration, base.Store:
		return nil
	default:
		p, _ := f.New(typ)
		return p
	}
}

func (ch *serveChild) setupControllers() {
	ch.controllers = map[base.Types]*services.PluginController{}
	factory := services.ControllerFactory(ch.ring, ch.signPrivKey, ch.store, ch.consulRegistry, ch.logger)
	for typ := range base.Types2Names {
		switch typ {
		case base.Store, base.Configuration:
		default:
			ch.controllers[typ] = setupController(factory, typ)
		}
	}
}

// StartControllers starts all the processes that produce syslog messages.
func (ch *serveChild) StartControllers() eerrors.ErrorSlice {
	funcs := make([]utils.Func, 0, len(base.Types2Names))
	for t, n := range base.Types2Names {
		typ := t
		name := n
		switch typ {
		case base.Store, base.Configuration:
		default:
			funcs = append(funcs, func() (err error) {
				return eerrors.Wrapf(ch.StartController(typ), "Error starting controller '%s'", name)
			})
		}
	}
	return utils.All(funcs...)
}

func (ch *serveChild) StartController(typ base.Types) error {
	switch typ {
	case base.RELP:
		return ch.StartRelp()
	case base.DirectRELP:
		return ch.StartDirectRelp()
	case base.TCP:
		return ch.StartTcp()
	case base.UDP:
		return ch.StartUdp()
	case base.Graylog:
		return ch.StartGraylog()
	case base.Journal:
		return ch.StartJournal()
	case base.Accounting:
		return ch.StartAccounting()
	case base.MacOS:
		return ch.StartMacOS()
	case base.KafkaSource:
		return ch.StartKafkaSource()
	case base.Filesystem:
		return ch.StartFSPoll()
	case base.HTTPServer:
		return ch.StartHTTPServer()
	default:
		return nil
	}
}

func (ch *serveChild) StartHTTPServer() error {
	if len(ch.conf.HTTPServerSource) == 0 {
		return nil
	}
	certfiles := ch.conf.GetCertificateFiles()["httpserversource"]
	certpaths := ch.conf.GetCertificatePaths()["httpserversource"]

	ctl := ch.controllers[base.HTTPServer]
	err := ctl.Create(
		services.DumpableOpt(DumpableFlag),
		services.CertFilesOpt(certfiles),
		services.CertPathsOpt(certpaths),
	)

	if err != nil {
		return eerrors.Wrap(err, "Error creating HTTP server controller")
	}
	ctl.SetConf(*ch.conf)
	_, err = ctl.Start()
	if err != nil {
		return eerrors.Wrap(err, "Error starting HTTP server controller")
	}
	ch.logger.Debug("HTTP server plugin has been started")
	return nil
}

func (ch *serveChild) StartFSPoll() error {
	if len(ch.conf.FSSource) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(ch.conf.FSSource))
	for _, source := range ch.conf.FSSource {
		if utils.IsDir(source.BaseDirectory) {
			dirs = append(dirs, source.BaseDirectory)
		}
	}
	if len(dirs) > 0 {
		ch.logger.Info("FS polling is enabled")
		err := ch.controllers[base.Filesystem].Create(
			services.DumpableOpt(DumpableFlag),
			services.PollDirectories(dirs),
		)
		if err != nil {
			return eerrors.Wrap(err, "Error creating fspoll controller")
		}
		ch.controllers[base.Filesystem].SetConf(*ch.conf)
		_, err = ch.controllers[base.Filesystem].Start()
		if err != nil {
			return eerrors.Wrap(err, "Error starting fspoll controller")
		}
		ch.logger.Debug("fspoll plugin has been started")
	}
	return nil
}

func (ch *serveChild) StartKafkaSource() error {
	if len(ch.conf.KafkaSource) > 0 {
		ch.logger.Info("Kafka sources are enabled")
		certfiles := ch.conf.GetCertificateFiles()["kafkasource"]
		certpaths := ch.conf.GetCertificatePaths()["kafkasource"]

		err := ch.controllers[base.KafkaSource].Create(
			services.DumpableOpt(DumpableFlag),
			services.CertFilesOpt(certfiles),
			services.CertPathsOpt(certpaths),
		)
		if err != nil {
			return eerrors.Wrap(err, "Error creating Kafka controller")
		}
		ch.controllers[base.KafkaSource].SetConf(*ch.conf)
		_, err = ch.controllers[base.KafkaSource].Start()
		if err != nil {
			return eerrors.Wrap(err, "Error starting Kafka controller")
		}
		ch.logger.Debug("Kafka source plugin has been started")
	}
	return nil
}

// StartAccounting starts the Accounting process.
func (ch *serveChild) StartAccounting() error {
	if ch.conf.Accounting.Enabled {
		ch.logger.Info("Process accounting is enabled")
		err := ch.controllers[base.Accounting].Create(
			services.DumpableOpt(DumpableFlag),
			services.AccountingPathOpt(ch.conf.Accounting.Path),
		)
		if err != nil {
			return eerrors.Wrap(err, "Error creating accounting controller")
		}
		ch.controllers[base.Accounting].SetConf(*ch.conf)
		_, err = ch.controllers[base.Accounting].Start()
		if err != nil {
			return eerrors.Wrap(err, "Error starting accounting controller")
		}
		ch.logger.Debug("Accounting plugin has been started")
	}
	return nil
}

// StartMacOS starts the MacOS logs process.
func (ch *serveChild) StartMacOS() error {
	if ch.conf.MacOS.Enabled && macos.Supported {
		ch.logger.Info("macos logs source is enabled")
		err := ch.controllers[base.MacOS].Create(
			services.DumpableOpt(DumpableFlag),
		)
		if err != nil {
			return eerrors.Wrap(err, "Error creating macos controller")
		}
		ch.controllers[base.MacOS].SetConf(*ch.conf)
		_, err = ch.controllers[base.MacOS].Start()
		if err != nil {
			return eerrors.Wrap(err, "Error starting macos controller")
		}
		ch.logger.Debug("macos plugin has been started")
	}
	return nil
}

// StartJournal starts the journald process.
func (ch *serveChild) StartJournal() error {
	if journald.Supported {
		ch.logger.Info("Journald is supported")
		if ch.conf.Journald.Enabled {
			ctl := ch.controllers[base.Journal]
			ch.logger.Info("Journald service is enabled")
			// in fact Create() will only do something the first time startJournal() is called
			err := ctl.Create(
				services.DumpableOpt(DumpableFlag),
			)
			if err != nil {
				return eerrors.Wrap(err, "Error creating journald controller")
			}
			ctl.SetConf(*ch.conf)
			_, err = ctl.Start()
			if err != nil {
				return eerrors.Wrap(err, "Error starting journald controller")
			}
			ch.logger.Debug("Journald plugin has been started")
		} else {
			ch.logger.Info("Journald service is disabled")
		}
	} else {
		ch.logger.Info("Journald service is not supported (only Linux)")
	}
	return nil
}

// StartRelp starts the Relp process.
func (ch *serveChild) StartRelp() error {
	if len(ch.conf.RELPSource) == 0 {
		return nil
	}
	certfiles := ch.conf.GetCertificateFiles()["relpsource"]
	certpaths := ch.conf.GetCertificatePaths()["relpsource"]

	ctl := ch.controllers[base.RELP]
	err := ctl.Create(
		services.DumpableOpt(DumpableFlag),
		services.CertFilesOpt(certfiles),
		services.CertPathsOpt(certpaths),
	)

	if err != nil {
		return eerrors.Wrap(err, "Error creating RELP controller")
	}
	ctl.SetConf(*ch.conf)
	_, err = ctl.Start()
	if err != nil {
		return eerrors.Wrap(err, "Error starting RELP controller")
	}
	ch.logger.Debug("RELP plugin has been started")
	return nil
}

// StartDirectRelp starts the DirectRelp process.
func (ch *serveChild) StartDirectRelp() error {
	if len(ch.conf.DirectRELPSource) == 0 {
		return nil
	}
	certfiles := ch.conf.GetCertificateFiles()["directrelpsource"]
	certpaths := ch.conf.GetCertificatePaths()["directrelpsource"]

	ctl := ch.controllers[base.DirectRELP]
	err := ctl.Create(
		services.DumpableOpt(DumpableFlag),
		services.CertFilesOpt(certfiles),
		services.CertPathsOpt(certpaths),
	)

	if err != nil {
		return eerrors.Wrap(err, "Error creating DirectRELP controller")
	}
	ctl.SetConf(*ch.conf)
	_, err = ctl.Start()
	if err != nil {
		return eerrors.Wrap(err, "Error starting DirectRELP controller")
	}
	ch.logger.Debug("DirectRELP plugin has been started")
	return nil
}

// StartTcp starts the TCP process.
func (ch *serveChild) StartTcp() error {
	if len(ch.conf.TCPSource) == 0 {
		return nil
	}
	certfiles := ch.conf.GetCertificateFiles()["tcpsource"]
	certpaths := ch.conf.GetCertificatePaths()["tcpsource"]

	ctl := ch.controllers[base.TCP]
	err := ctl.Create(
		services.DumpableOpt(DumpableFlag),
		services.CertFilesOpt(certfiles),
		services.CertPathsOpt(certpaths),
	)

	if err != nil {
		return eerrors.Wrap(err, "Error creating TCP controller")
	}
	ctl.SetConf(*ch.conf)
	tcpinfos, err := ctl.Start()
	if err == services.NOLISTENER {
		ch.logger.Info("TCP plugin not started")
	} else if err != nil {
		return eerrors.Wrap(err, "Error starting TCP controller")
	} else if len(tcpinfos) == 0 {
		ch.logger.Info("TCP plugin not started")
	} else {
		ch.logger.Debug("TCP plugin has been started", "listeners", len(tcpinfos))
	}
	return nil
}

// StartUdp starts the UDP process.
func (ch *serveChild) StartUdp() error {
	if len(ch.conf.UDPSource) == 0 {
		return nil
	}
	ctl := ch.controllers[base.UDP]
	err := ctl.Create(
		services.DumpableOpt(DumpableFlag),
	)

	if err != nil {
		return eerrors.Wrap(err, "Error creating UDP controller")
	}
	ctl.SetConf(*ch.conf)
	udpinfos, err := ctl.Start()
	if err == services.NOLISTENER {
		ch.logger.Info("UDP plugin not started")
	} else if err != nil {
		return eerrors.Wrap(err, "Error starting UDP controller")
	} else if len(udpinfos) == 0 {
		ch.logger.Info("UDP plugin not started")
	} else {
		ch.logger.Debug("UDP plugin started", "listeners", len(udpinfos))
	}
	return nil
}

// StartGraylog starts the Graylog process.
func (ch *serveChild) StartGraylog() error {
	if len(ch.conf.GraylogSource) == 0 {
		return nil
	}
	ctl := ch.controllers[base.Graylog]
	err := ctl.Create(
		services.DumpableOpt(DumpableFlag),
	)

	if err != nil {
		return eerrors.Wrap(err, "Error creating Graylog controller")
	}
	ctl.SetConf(*ch.conf)
	infos, err := ctl.Start()
	if err == services.NOLISTENER {
		ch.logger.Info("Graylog plugin not started")
	} else if err != nil {
		return eerrors.Wrap(err, "Error starting Graylog controller")
	} else if len(infos) == 0 {
		ch.logger.Info("Graylog plugin not started")
	} else {
		ch.logger.Debug("Graylog plugin started", "listeners", len(infos))
	}
	return nil
}

// StopController stops a process of specified type.
func (ch *serveChild) StopController(typ base.Types, doShutdown bool) error {
	switch typ {
	case base.Store, base.Configuration:
		return nil
	case base.Journal:
		if journald.Supported {
			if doShutdown {
				if ch.controllers[base.Journal].Shutdown(5 * time.Second) {
					return eerrors.Errorf("Controller '%s' has been killed", base.Types2Names[typ])
				}
				return nil
			}
			// we keep the same instance of the journald plugin, so
			// that we can continue to fetch messages from a
			// consistent position in journald
			return ch.controllers[base.Journal].Stop()
		}
	}
	if ch.controllers[typ].Shutdown(5 * time.Second) {
		return eerrors.Errorf("Controller '%s' has been killed", base.Types2Names[typ])
	}
	return nil
}

// Reload restarts all the plugin processes.
func (ch *serveChild) Reload() (err error) {
	ch.logger.Info("Reloading configuration and services")
	// first, let's stop the HTTP server that reports the metrics
	ch.metricsServer.Stop()
	// stop the kafka forwarder
	ch.store.Stop()
	ch.logger.Debug("The forwarder has been stopped")
	ch.store.SetConf(*ch.conf)
	// restart the kafka forwarder
	_, err = ch.store.Start()
	if err != nil {
		return err
	}
	funcs := make([]utils.Func, 0, len(base.Types2Names))
	for t, n := range base.Types2Names {
		typ := t
		name := n
		switch typ {
		case base.Store, base.Configuration:
		default:
			funcs = append(funcs, func() (err error) {
				err = ch.StopController(typ, false)
				if err != nil {
					ch.logger.Warn("Error stopping controller", "type", name)
				}
				return eerrors.Wrapf(ch.StartController(typ), "Error restarting controller '%s'", name)
			})
		}
	}
	errs := utils.All(funcs...)
	if !errs.Empty() {
		return errs
	}

	ch.setupMetrics(ch.logger)
	return nil
}

func (ch *serveChild) setupMetrics(logger log15.Logger) {
	ch.metricsServer = &metrics.MetricsServer{}
	controllers := make([]prometheus.Gatherer, 0, len(base.Types2Names))
	for t := range base.Types2Names {
		typ := t
		switch typ {
		case base.Configuration:
		case base.Store:
			controllers = append(controllers, ch.store)
		default:
			controllers = append(controllers, ch.controllers[typ])
		}
	}
	ch.metricsServer.NewConf(ch.conf.Metrics, logger, controllers...)
}

// Serve starts the controllers and reacts to signals and events.
func (ch *serveChild) Serve() error {
	ch.logger.Debug("Serve() runs under user", "uid", os.Getuid(), "gid", os.Getgid())
	if capabilities.CapabilitiesSupported {
		ch.logger.Debug("Capabilities", "caps", capabilities.GetCaps())
	}

	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	errs := ch.StartControllers()
	if !errs.Empty() {
		return errs.Wrap("Error starting controllers")
	}

	ch.logger.Debug("Main loop is starting")
	c := eerrors.ChainErrors()

	go func() {
		// if parent disappears for any reason, we shutdown
		deadManPipe := os.NewFile(uintptr(len(base.Handles)+4), "deadmanpipe")
		dummy := make([]byte, 1)
		deadManPipe.Read(dummy)
		c.Append(eerrors.New("Parent has gone!!! Shutting down"))
		ch.shutdown()
	}()

	go func() {
		<-ch.store.ShutdownChan
		c.Append(errors.New("Store has shutdown: aborting all operations"))
		ch.shutdown()
	}()

	for {
		select {
		case <-ch.shutdownCtx.Done():
			ch.logger.Info("Shutting down")
			errs := c.Sum()
			if !errs.Empty() {
				return errs
			}
			return nil
		default:
		}

		select {
		case <-ch.shutdownCtx.Done():
			// just loop
		case newConf := <-ch.confChan:
			if newConf != nil {
				// some parameters can't be modified online
				newConf.Store = ch.conf.Store
				newConf.Main.EncryptIPC = ch.conf.Main.EncryptIPC
				ch.conf = newConf
				err := ch.Reload()
				if err != nil {
					c.Append(eerrors.Wrap(err, "Fatal error when restarting services"))
					ch.shutdown()
				}
			} else {
				ch.logger.Debug("Configuration channel is closed")
				ch.shutdown()
			}

		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				signal.Stop(sigChan)
				signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
				select {
				case <-ch.shutdownCtx.Done():
				default:
					ch.logger.Info("SIGHUP received: reloading configuration")
					err := ch.confService.Reload()
					sigChan = make(chan os.Signal, 10)
					signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
					if err != nil {
						c.Append(eerrors.Wrap(err, "Error reloading configuration service"))
						ch.shutdown()
					}
				}
			case syscall.SIGTERM, syscall.SIGINT:
				signal.Stop(sigChan)
				signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
				sigChan = nil
				ch.logger.Info("Termination signal received", "signal", sig)
				ch.shutdown()
			default:
				ch.logger.Info("Unknown signal received", "signal", sig)
			}
		}
	}
}
