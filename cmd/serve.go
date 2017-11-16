package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/logging"
)

var ServeCmd = &cobra.Command{
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
var Handles []string
var HandlesMap map[string]int

func init() {
	RootCmd.AddCommand(ServeCmd)
	ServeCmd.Flags().BoolVar(&testFlag, "test", false, "Print messages to stdout instead of sending to Kafka")
	ServeCmd.Flags().BoolVar(&SyslogFlag, "syslog", false, "Send logs to the local syslog (are you sure you wan't to do that ?)")
	ServeCmd.Flags().StringVar(&LoglevelFlag, "loglevel", "info", "Set logging level")
	ServeCmd.Flags().StringVar(&LogfilenameFlag, "logfilename", "", "Write logs to a file instead of stderr")
	ServeCmd.Flags().BoolVar(&LogjsonFlag, "logjson", false, "Write logs in JSON format")
	ServeCmd.Flags().StringVar(&pidFilenameFlag, "pidfile", "", "If given, write PID to file")
	ServeCmd.Flags().BoolVar(&consulRegisterFlag, "register", false, "Register services in consul")
	ServeCmd.Flags().StringVar(&consulServiceName, "servicename", "skewer", "Service name to register in consul")
	ServeCmd.Flags().StringVar(&UidFlag, "uid", "", "Switch to this user ID (when launched as root)")
	ServeCmd.Flags().StringVar(&GidFlag, "gid", "", "Switch to this group ID (when launched as root)")
	ServeCmd.Flags().BoolVar(&DumpableFlag, "dumpable", false, "if set, the skewer process will be traceable/dumpable")
	ServeCmd.Flags().BoolVar(&profile, "profile", false, "if set, profile memory")

	Handles = []string{
		"CHILD_BINDER",
		"TCP_BINDER",
		"UDP_BINDER",
		"RELP_BINDER",
		"CHILD_LOGGER",
		"TCP_LOGGER",
		"UDP_LOGGER",
		"RELP_LOGGER",
		"JOURNAL_LOGGER",
		"CONFIG_LOGGER",
		"STORE_LOGGER",
		"ACCT_LOGGER",
	}

	HandlesMap = map[string]int{}
	for i, h := range Handles {
		HandlesMap[h] = i + 3
	}
}

func ExecuteChild() (err error) {
	ch := NewServeChild()
	err = ch.Init()
	if err != nil {
		return fmt.Errorf("Fatal error initializing Serve: %s", err)
	}
	defer ch.Cleanup()
	err = ch.Serve()
	if err != nil {
		return fmt.Errorf("Fatal error executing Serve: %s", err)
	}
	return nil
}

type ServeChild struct {
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
	controllers    map[services.NetworkServiceType]*services.PluginController
	metricsServer  *metrics.MetricsServer
}

func NewServeChild() *ServeChild {
	c := ServeChild{}
	c.globalCtx, c.globalCancel = context.WithCancel(context.Background())
	c.shutdownCtx, c.shutdown = context.WithCancel(c.globalCtx)
	loggerConn, _ := net.FileConn(os.NewFile(uintptr(HandlesMap["CHILD_LOGGER"]), "logger"))
	loggerConn.(*net.UnixConn).SetReadBuffer(65536)
	loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
	c.loggerCtx, c.cancelLogger = context.WithCancel(context.Background())
	c.logger = logging.NewRemoteLogger(c.loggerCtx, loggerConn).New("proc", "child")
	return &c
}

func (ch *ServeChild) Init() error {
	err := ch.SetupConsulRegistry()
	if err != nil {
		return err
	}
	err = ch.SetupConfiguration()
	if err != nil {
		return err
	}
	err = ch.SetupStore()
	if err != nil {
		return err
	}
	ch.controllers = map[services.NetworkServiceType]*services.PluginController{}
	ch.SetupControllers()
	ch.SetupMetrics(ch.logger)
	return nil
}

func (ch *ServeChild) Cleanup() {
	ch.ShutdownControllers()
	ch.cancelLogger()
	ch.shutdown()
	ch.globalCancel()
}

func (ch *ServeChild) SetupConfiguration() error {
	ch.confService = services.NewConfigurationService(HandlesMap["CONFIG_LOGGER"], ch.logger)
	ch.confService.SetConfDir(configDirName)
	ch.confService.SetConsulParams(ch.consulParams)
	err := ch.confService.Start()
	if err != nil {
		return fmt.Errorf("Error starting the configuration service: %s", err)
	}
	ch.confChan = ch.confService.Chan()
	if ch.confChan == nil {
		return fmt.Errorf("Error starting the configuration service")
	}
	ch.conf = <-ch.confChan
	ch.conf.Store.Dirname = storeDirname
	ch.logger.Info("Store location", "path", ch.conf.Store.Dirname)
	return nil
}

func (ch *ServeChild) SetupConsulRegistry() error {
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

func (ch *ServeChild) SetupStore() error {
	// setup the Store
	ch.store = services.NewStorePlugin(HandlesMap["STORE_LOGGER"], ch.logger)
	ch.store.SetConf(*ch.conf)
	err := ch.store.Create(testFlag, DumpableFlag, storeDirname, "", "")
	if err != nil {
		return fmt.Errorf("Can't create the message Store: %s", err)
	}
	go func() {
		<-ch.store.ShutdownChan
		ch.logger.Info("Store has shutdown: aborting all operations")
		ch.shutdown()
	}()
	_, err = ch.store.Start()
	if err != nil {
		return fmt.Errorf("Can't start the forwarder: %s", err)
	}
	return nil
}

func (ch *ServeChild) SetupController(typ services.NetworkServiceType) {
	var binder int
	var logger int
	switch typ {
	case services.RELP:
		binder = HandlesMap["RELP_BINDER"]
		logger = HandlesMap["RELP_LOGGER"]
	case services.TCP:
		binder = HandlesMap["TCP_BINDER"]
		logger = HandlesMap["TCP_LOGGER"]
	case services.UDP:
		binder = HandlesMap["UDP_BINDER"]
		logger = HandlesMap["UDP_LOGGER"]
	case services.Journal:
		logger = HandlesMap["JOURNAL_LOGGER"]
	case services.Accounting:
		logger = HandlesMap["ACCT_LOGGER"]
	}
	ch.controllers[typ] = services.NewPluginController(
		typ, ch.store,
		ch.consulRegistry,
		binder, logger,
		ch.logger,
	)
}

func (ch *ServeChild) SetupControllers() {
	ch.SetupController(services.RELP)
	ch.SetupController(services.TCP)
	ch.SetupController(services.UDP)
	ch.SetupController(services.Journal)
	ch.SetupController(services.Accounting)
}

func (ch *ServeChild) StartControllers() error {
	return utils.Parallel(
		ch.StartRelp,
		ch.StartTcp,
		ch.StartUdp,
		ch.StartJournal,
		ch.StartAccounting,
	)
}

func (ch *ServeChild) StartAccounting() error {
	if ch.conf.Accounting.Enabled {
		ch.logger.Info("Process accounting is enabled")
		err := ch.controllers[services.Accounting].Create(testFlag, DumpableFlag, "", "", ch.conf.Accounting.Path)
		if err != nil {
			return fmt.Errorf("Error creating the accounting plugin: %s", err)
		}
		ch.controllers[services.Accounting].SetConf(*ch.conf)
		_, err = ch.controllers[services.Accounting].Start()
		if err != nil {
			return fmt.Errorf("Error starting accounting plugin: %s", err)
		} else {
			ch.logger.Debug("Accounting plugin has been started")
		}
	}
	return nil
}

func (ch *ServeChild) StartJournal() error {
	if journald.Supported {
		ch.logger.Info("Journald is supported")
		if ch.conf.Journald.Enabled {
			ctl := ch.controllers[services.Journal]
			ch.logger.Info("Journald service is enabled")
			// in fact Create() will only do something the first time startJournal() is called
			err := ctl.Create(testFlag, DumpableFlag, "", "", "")
			if err != nil {
				return fmt.Errorf("Error creating Journald plugin: %s", err)
			}
			ctl.SetConf(*ch.conf)
			_, err = ctl.Start()
			if err != nil {
				return fmt.Errorf("Error starting Journald plugin: %s", err)
			} else {
				ch.logger.Debug("Journald plugin has been started")
			}
		} else {
			ch.logger.Info("Journald service is disabled")
		}
	} else {
		ch.logger.Info("Journald service is not supported (only Linux)")
	}
	return nil
}

func (ch *ServeChild) StartRelp() error {
	ctl := ch.controllers[services.RELP]
	err := ctl.Create(testFlag, DumpableFlag, "", "", "")
	if err != nil {
		return fmt.Errorf("Error creating RELP plugin: %s", err)
	}
	ctl.SetConf(*ch.conf)
	_, err = ctl.Start()
	if err != nil {
		return fmt.Errorf("Error starting RELP plugin: %s", err)
	} else {
		ch.logger.Debug("RELP plugin has been started")
	}
	return nil
}

func (ch *ServeChild) StartTcp() error {
	ctl := ch.controllers[services.TCP]
	err := ctl.Create(testFlag, DumpableFlag, "", "", "")
	if err != nil {
		return fmt.Errorf("Error creating TCP plugin: %s", err)
	}
	ctl.SetConf(*ch.conf)
	tcpinfos, err := ctl.Start()
	if err != nil {
		return fmt.Errorf("Error starting TCP plugin: %s", err)
	} else if len(tcpinfos) == 0 {
		ch.logger.Info("TCP plugin not started")
	} else {
		ch.logger.Debug("TCP plugin has been started", "listeners", len(tcpinfos))
	}
	return nil
}

func (ch *ServeChild) StartUdp() error {
	ctl := ch.controllers[services.UDP]
	err := ctl.Create(testFlag, DumpableFlag, "", "", "")
	if err != nil {
		return fmt.Errorf("Error creating UDP plugin: %s", err)
	}
	ctl.SetConf(*ch.conf)
	udpinfos, err := ctl.Start()
	if err != nil {
		return fmt.Errorf("Error starting UDP plugin: %s", err)
	} else if len(udpinfos) == 0 {
		ch.logger.Info("UDP plugin not started")
	} else {
		ch.logger.Debug("UDP plugin started", "listeners", len(udpinfos))
	}
	return nil
}

func (ch *ServeChild) StopController(typ services.NetworkServiceType, doShutdown bool) {
	switch typ {
	case services.TCP, services.UDP, services.RELP, services.Accounting:
		ch.controllers[typ].Shutdown(5 * time.Second)
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
	}
}

func (ch *ServeChild) Reload() (err error) {
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
	err = utils.Parallel(
		func() error {
			if !journald.Supported {
				return nil
			}
			ch.StopController(services.Journal, false)
			return ch.StartJournal()
		},
		func() error {
			ch.StopController(services.Accounting, false)
			return ch.StartAccounting()
		},
		func() error {
			ch.StopController(services.RELP, false)
			return ch.StartRelp()
		},
		func() error {
			ch.StopController(services.TCP, false)
			return ch.StartTcp()
		},
		func() error {
			ch.StopController(services.UDP, false)
			return ch.StartUdp()
		},
	)
	if err != nil {
		return err
	}

	ch.SetupMetrics(ch.logger)
	return nil
}

func (ch *ServeChild) SetupMetrics(logger log15.Logger) {
	ch.metricsServer = &metrics.MetricsServer{}
	ch.metricsServer.NewConf(
		ch.conf.Metrics,
		logger,
		ch.controllers[services.Journal],
		ch.controllers[services.RELP],
		ch.controllers[services.TCP],
		ch.controllers[services.UDP],
		ch.controllers[services.Accounting],
		ch.store,
	)
}

func (ch *ServeChild) ShutdownControllers() {
	utils.Parallel(
		func() error {
			ch.StopController(services.RELP, true)
			return nil
		},
		func() error {
			ch.StopController(services.Accounting, true)
			return nil
		},
		func() error {
			ch.StopController(services.Journal, true)
			return nil
		},
		func() error {
			ch.StopController(services.TCP, true)
			return nil
		},
		func() error {
			ch.StopController(services.UDP, true)
			return nil
		},
	)
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
	ch.cancelLogger()
	time.Sleep(time.Second)
}

func (ch *ServeChild) Serve() error {
	ch.logger.Debug("Serve() runs under user", "uid", os.Getuid(), "gid", os.Getgid())
	if capabilities.CapabilitiesSupported {
		ch.logger.Debug("Capabilities", "caps", capabilities.GetCaps())
	}

	sig_chan := make(chan os.Signal, 10)
	signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	err := ch.StartControllers()
	if err != nil {
		return fmt.Errorf("Error starting a controller: %s", err)
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
			server.ListenAndServe()
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
				newConf.Store = ch.conf.Store
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

		case sig := <-sig_chan:
			switch sig {
			case syscall.SIGHUP:
				signal.Stop(sig_chan)
				signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
				select {
				case <-ch.shutdownCtx.Done():
				default:
					ch.logger.Info("SIGHUP received: reloading configuration")
					ch.confService.Reload()
					sig_chan = make(chan os.Signal, 10)
					signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
				}
			case syscall.SIGTERM, syscall.SIGINT:
				signal.Stop(sig_chan)
				signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
				sig_chan = nil
				ch.logger.Info("Termination signal received", "signal", sig)
				ch.shutdown()
			default:
				ch.logger.Info("Unknown signal received", "signal", sig)
			}

		}
	}
}
