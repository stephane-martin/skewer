package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/utils"
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

var testFlag bool
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
	serveCobraCmd.Flags().BoolVar(&testFlag, "test", false, "Print messages to stdout instead of sending to Kafka")
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
	serveCobraCmd.Flags().BoolVar(&profile, "profile", false, "if set, profile memory")

}

// ExecuteChild sets up the environment for the serve command and starts it.
func ExecuteChild() (err error) {
	sessionID := strings.TrimSpace(os.Getenv("SKEWER_SESSION"))
	if len(sessionID) == 0 {
		return fmt.Errorf("empty session ID")
	}
	ringSecretPipe := os.NewFile(uintptr(len(services.Handles)+3), "ringsecretpipe")
	var ringSecret *memguard.LockedBuffer
	buf := make([]byte, 32)
	_, err = ringSecretPipe.Read(buf)
	if err != nil {
		return err
	}
	ringSecret, err = memguard.NewImmutableFromBytes(buf)
	if err != nil {
		return err
	}
	creds := kring.RingCreds{Secret: ringSecret, SessionID: ulid.MustParse(sessionID)}
	ring := kring.GetRing(creds)
	secret, err := ring.GetBoxSecret()
	if err != nil {
		return err
	}
	ch, err := newServeChild(ring)
	if err != nil {
		return fmt.Errorf("fatal error initializing main child: %s", err)
	}
	err = ch.init()
	if err != nil {
		return fmt.Errorf("fatal error initializing Serve: %s", err)
	}
	defer func() {
		ch.cleanup()
		secret.Destroy()
	}()
	err = ch.Serve()
	if err != nil {
		return fmt.Errorf("fatal error executing Serve: %s", err)
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
	controllers    map[services.Types]*services.PluginController
	metricsServer  *metrics.MetricsServer
	signPrivKey    *memguard.LockedBuffer
	ring           kring.Ring
}

func newServeChild(ring kring.Ring) (*serveChild, error) {
	secret, err := ring.GetBoxSecret()
	if err != nil {
		return nil, err
	}
	childLoggerHdl := services.HandlesMap[services.ServiceHandle{"child", services.LOGGER}]
	conn, err := net.FileConn(os.NewFile(childLoggerHdl, "logger"))
	if err != nil {
		return nil, err
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
		return err
	}

	err = ch.setupSignKey()
	if err != nil {
		return err
	}

	err = ch.setupConfiguration()
	if err != nil {
		return err
	}

	st, err := ch.setupStore()
	if err != nil {
		return err
	}
	ch.store = st

	ch.setupControllers()
	ch.setupMetrics(ch.logger)
	return nil
}

func (ch *serveChild) cleanup() {
	ch.ShutdownControllers()
	ch.cancelLogger()
	ch.shutdown()
	ch.globalCancel()
	if ch.signPrivKey != nil {
		ch.signPrivKey.Destroy()
	}
	_ = ch.ring.Destroy()
}

// ShutdownControllers definitely shutdowns the plugin processes.
func (ch *serveChild) ShutdownControllers() {
	funcs := make([]utils.Func, 0, len(services.Names2Types))
	for _, t := range services.Names2Types {
		typ := t
		switch typ {
		case services.Store, services.Configuration:
			// shutdown them later
		default:
			funcs = append(funcs, func() error {
				ch.StopController(typ, true)
				return nil
			})
		}
	}
	err := utils.Parallel(funcs...)
	if err != nil {
		ch.logger.Error("Error shutting down controllers", "error", err)
	}

	ch.logger.Debug("The RELP service has been stopped")
	ch.logger.Debug("Stopped accounting service")
	ch.logger.Debug("Stopped journald service")
	ch.logger.Debug("The TCP service has been stopped")
	ch.logger.Debug("The UDP service has been stopped")

	ch.globalCancel()
	ch.store.Shutdown(5 * time.Second)
	if ch.consulRegistry != nil {
		ch.consulRegistry.WaitFinished() // wait that the services have been unregistered from Consul
	}
	ch.confService.Stop()
}

func (ch *serveChild) setupConfiguration() error {
	ch.confService = services.NewConfigurationService(ch.signPrivKey, services.LoggerHdl(services.Configuration), ch.logger)
	ch.confService.SetConfDir(configDirName)
	ch.confService.SetConsulParams(ch.consulParams)
	err := ch.confService.Start(ch.ring)
	if err != nil {
		return fmt.Errorf("error starting the configuration service: %s", err)
	}
	ch.confChan = ch.confService.Chan()
	if ch.confChan == nil {
		return fmt.Errorf("error starting the configuration service")
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
		Prefix:     consulPrefix,
	}
	var err error
	if consulRegisterFlag {
		ch.consulRegistry, err = consul.NewRegistry(ch.globalCtx, ch.consulParams, consulServiceName, ch.logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ch *serveChild) setupStore() (st *services.StorePlugin, err error) {
	f := services.ControllerFactory(ch.ring, ch.signPrivKey, nil, ch.consulRegistry, ch.logger)
	st = f.NewStore(services.LoggerHdl(services.Store))
	st.SetConf(*ch.conf)

	tmpl := ""
	dests, _ := ch.conf.Main.GetDestinations()
	if (dests & conf.File) != 0 {
		tmpl = ch.conf.FileDest.Filename
	}

	certfiles := ch.conf.GetCertificateFiles()["dests"]
	certpaths := ch.conf.GetCertificatePaths()["dests"]

	err = st.Create(
		services.TestOpt(testFlag),
		services.DumpableOpt(DumpableFlag),
		services.StorePathOpt(storeDirname),
		services.FileDestTmplOpt(tmpl),
		services.CertFilesOpt(certfiles),
		services.CertPathsOpt(certpaths),
	)
	if err != nil {
		return nil, fmt.Errorf("can't create the message Store: %s", err)
	}
	go func() {
		<-st.ShutdownChan
		ch.logger.Info("Store has shutdown: aborting all operations")
		ch.shutdown()
	}()
	_, err = st.Start()
	if err != nil {
		return nil, fmt.Errorf("can't start the forwarder: %s", err)
	}
	return st, nil
}

func (ch *serveChild) setupSignKey() error {
	ch.logger.Debug("Generating signature keys")
	privkey, err := ch.ring.NewSignaturePubkey()
	if err != nil {
		return fmt.Errorf("error generating signature keys: %s", err)
	}
	ch.signPrivKey = privkey
	return nil
}

func setupController(f *services.CFactory, typ services.Types) *services.PluginController {
	switch typ {
	case services.Configuration, services.Store:
		return nil
	default:
		return f.New(typ)
	}
}

func (ch *serveChild) setupControllers() {
	ch.controllers = map[services.Types]*services.PluginController{}
	factory := services.ControllerFactory(ch.ring, ch.signPrivKey, ch.store, ch.consulRegistry, ch.logger)
	for typ := range services.Types2Names {
		switch typ {
		case services.Store, services.Configuration:
		default:
			ch.controllers[typ] = setupController(factory, typ)
		}
	}
}

// StartControllers starts all the processes that produce syslog messages.
func (ch *serveChild) StartControllers() error {
	funcs := make([]utils.Func, 0, len(services.Types2Names))
	for t := range services.Types2Names {
		typ := t
		switch typ {
		case services.Store, services.Configuration:
		default:
			funcs = append(funcs, func() error {
				return ch.StartController(typ)
			})
		}
	}
	return utils.All(funcs...)
}

func (ch *serveChild) StartController(typ services.Types) error {
	switch typ {
	case services.RELP:
		return ch.StartRelp()
	case services.TCP:
		return ch.StartTcp()
	case services.UDP:
		return ch.StartUdp()
	case services.Journal:
		return ch.StartJournal()
	case services.Accounting:
		return ch.StartAccounting()
	case services.KafkaSource:
		return ch.StartKafkaSource()
	default:
		return nil
	}
}

func (ch *serveChild) StartKafkaSource() error {
	if len(ch.conf.KafkaSource) > 0 {
		ch.logger.Info("Kafka sources are enabled")
		certfiles := ch.conf.GetCertificateFiles()["kafkasource"]
		certpaths := ch.conf.GetCertificatePaths()["kafkasource"]

		err := ch.controllers[services.KafkaSource].Create(
			services.TestOpt(testFlag),
			services.DumpableOpt(DumpableFlag),
			services.CertFilesOpt(certfiles),
			services.CertPathsOpt(certpaths),
		)
		if err != nil {
			return fmt.Errorf("error creating the kafka source plugin: %s", err)
		}
		ch.controllers[services.KafkaSource].SetConf(*ch.conf)
		_, err = ch.controllers[services.KafkaSource].Start()
		if err != nil {
			return fmt.Errorf("error starting the kafka source plugin: %s", err)
		}
		ch.logger.Debug("Kafka source plugin has been started")
	}
	return nil
}

// StartAccounting starts the Accounting process.
func (ch *serveChild) StartAccounting() error {
	if ch.conf.Accounting.Enabled {
		ch.logger.Info("Process accounting is enabled")
		err := ch.controllers[services.Accounting].Create(
			services.TestOpt(testFlag),
			services.DumpableOpt(DumpableFlag),
			services.AccountingPathOpt(ch.conf.Accounting.Path),
		)
		if err != nil {
			return fmt.Errorf("error creating the accounting plugin: %s", err)
		}
		ch.controllers[services.Accounting].SetConf(*ch.conf)
		_, err = ch.controllers[services.Accounting].Start()
		if err != nil {
			return fmt.Errorf("error starting accounting plugin: %s", err)
		}
		ch.logger.Debug("Accounting plugin has been started")
	}
	return nil
}

// StartJournal starts the journald process.
func (ch *serveChild) StartJournal() error {
	if journald.Supported {
		ch.logger.Info("Journald is supported")
		if ch.conf.Journald.Enabled {
			ctl := ch.controllers[services.Journal]
			ch.logger.Info("Journald service is enabled")
			// in fact Create() will only do something the first time startJournal() is called
			err := ctl.Create(
				services.TestOpt(testFlag),
				services.DumpableOpt(DumpableFlag),
			)
			if err != nil {
				return fmt.Errorf("error creating Journald plugin: %s", err)
			}
			ctl.SetConf(*ch.conf)
			_, err = ctl.Start()
			if err != nil {
				return fmt.Errorf("error starting Journald plugin: %s", err)
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
	certfiles := ch.conf.GetCertificateFiles()["relpsource"]
	certpaths := ch.conf.GetCertificatePaths()["relpsource"]

	ctl := ch.controllers[services.RELP]
	err := ctl.Create(
		services.TestOpt(testFlag),
		services.DumpableOpt(DumpableFlag),
		services.CertFilesOpt(certfiles),
		services.CertPathsOpt(certpaths),
	)

	if err != nil {
		return fmt.Errorf("error creating RELP plugin: %s", err)
	}
	ctl.SetConf(*ch.conf)
	_, err = ctl.Start()
	if err != nil {
		return fmt.Errorf("error starting RELP plugin: %s", err)
	}
	ch.logger.Debug("RELP plugin has been started")
	return nil
}

// StartTcp starts the TCP process.
func (ch *serveChild) StartTcp() error {
	certfiles := ch.conf.GetCertificateFiles()["tcpsource"]
	certpaths := ch.conf.GetCertificatePaths()["tcpsource"]

	ctl := ch.controllers[services.TCP]
	err := ctl.Create(
		services.TestOpt(testFlag),
		services.DumpableOpt(DumpableFlag),
		services.CertFilesOpt(certfiles),
		services.CertPathsOpt(certpaths),
	)

	if err != nil {
		return fmt.Errorf("error creating TCP plugin: %s", err)
	}
	ctl.SetConf(*ch.conf)
	tcpinfos, err := ctl.Start()
	if err == services.NOLISTENER {
		ch.logger.Info("TCP plugin not started")
	} else if err != nil {
		return fmt.Errorf("error starting TCP plugin: %s", err)
	} else if len(tcpinfos) == 0 {
		ch.logger.Info("TCP plugin not started")
	} else {
		ch.logger.Debug("TCP plugin has been started", "listeners", len(tcpinfos))
	}
	return nil
}

// StartUdp starts the UDP process.
func (ch *serveChild) StartUdp() error {
	ctl := ch.controllers[services.UDP]
	err := ctl.Create(
		services.TestOpt(testFlag),
		services.DumpableOpt(DumpableFlag),
	)

	if err != nil {
		return fmt.Errorf("error creating UDP plugin: %s", err)
	}
	ctl.SetConf(*ch.conf)
	udpinfos, err := ctl.Start()
	if err == services.NOLISTENER {
		ch.logger.Info("UDP plugin not started")
	} else if err != nil {
		return fmt.Errorf("error starting UDP plugin: %s", err)
	} else if len(udpinfos) == 0 {
		ch.logger.Info("UDP plugin not started")
	} else {
		ch.logger.Debug("UDP plugin started", "listeners", len(udpinfos))
	}
	return nil
}

// StopController stops a process of specified type.
func (ch *serveChild) StopController(typ services.Types, doShutdown bool) {
	switch typ {
	case services.Store, services.Configuration:
		return
	case services.Journal:
		if journald.Supported {
			if doShutdown {
				ch.controllers[services.Journal].Shutdown(5 * time.Second)
			} else {
				// we keep the same instance of the journald plugin, so
				// that we can continue to fetch messages from a
				// consistent position in journald
				ch.controllers[services.Journal].Stop()
			}
		}
	default:
		ch.controllers[typ].Shutdown(5 * time.Second)
	}
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
	funcs := make([]utils.Func, 0, len(services.Types2Names))
	for t := range services.Types2Names {
		typ := t
		switch typ {
		case services.Store, services.Configuration:
		default:
			funcs = append(funcs, func() error {
				ch.StopController(typ, false)
				return ch.StartController(typ)
			})
		}
	}
	err = utils.All(funcs...)
	if err != nil {
		return err
	}

	ch.setupMetrics(ch.logger)
	return nil
}

func (ch *serveChild) setupMetrics(logger log15.Logger) {
	ch.metricsServer = &metrics.MetricsServer{}
	controllers := make([]prometheus.Gatherer, 0, len(services.Types2Names))
	for t := range services.Types2Names {
		typ := t
		switch typ {
		case services.Configuration:
		case services.Store:
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

	err := ch.StartControllers()
	if err != nil {
		return fmt.Errorf("error starting a controller: %s", err)
	}

	if profile {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/pprof/heap", pprof.Handler("heap"))
			mux.Handle("/pprof/profile", http.HandlerFunc(pprof.Profile))
			server := &http.Server{
				Addr:    "127.0.0.1:6600",
				Handler: mux,
			}
			err = server.ListenAndServe()
			if err != nil {
				ch.logger.Warn("Error starting the pprof HTTP server", "error", err)
			}
		}()
	}

	ch.logger.Debug("Main loop is starting")
	for {
		select {
		case <-ch.shutdownCtx.Done():
			ch.logger.Info("Shutting down")
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
					ch.logger.Crit("Fatal error when reloading configuration", "error", err)
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
					ch.confService.Reload()
					sigChan = make(chan os.Signal, 10)
					signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
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
