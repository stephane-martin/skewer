package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/kardianos/osext"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/dumpable"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/logging"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start listening for Syslog messages and forward them to Kafka",
	Long: `The serve command is the main skewer command. It launches a long
running process that listens to syslog messages according to the configuration,
connects to Kafka, and forwards messages to Kafka.`,
	Run: func(cmd *cobra.Command, args []string) {
		runserve()
	},
}

type spair struct {
	child  int
	parent int
}

var testFlag bool
var syslogFlag bool
var loglevelFlag string
var logfilenameFlag string
var logjsonFlag bool
var pidFilenameFlag string
var consulRegisterFlag bool
var consulServiceName string
var uidFlag string
var gidFlag string
var dumpableFlag bool
var profile bool
var handles []string
var handlesMap map[string]int

func init() {
	RootCmd.AddCommand(serveCmd)
	serveCmd.Flags().BoolVar(&testFlag, "test", false, "Print messages to stdout instead of sending to Kafka")
	serveCmd.Flags().BoolVar(&syslogFlag, "syslog", false, "Send logs to the local syslog (are you sure you wan't to do that ?)")
	serveCmd.Flags().StringVar(&loglevelFlag, "loglevel", "info", "Set logging level")
	serveCmd.Flags().StringVar(&logfilenameFlag, "logfilename", "", "Write logs to a file instead of stderr")
	serveCmd.Flags().BoolVar(&logjsonFlag, "logjson", false, "Write logs in JSON format")
	serveCmd.Flags().StringVar(&pidFilenameFlag, "pidfile", "", "If given, write PID to file")
	serveCmd.Flags().BoolVar(&consulRegisterFlag, "register", false, "Register services in consul")
	serveCmd.Flags().StringVar(&consulServiceName, "servicename", "skewer", "Service name to register in consul")
	serveCmd.Flags().StringVar(&uidFlag, "uid", "", "Switch to this user ID (when launched as root)")
	serveCmd.Flags().StringVar(&gidFlag, "gid", "", "Switch to this group ID (when launched as root)")
	serveCmd.Flags().BoolVar(&dumpableFlag, "dumpable", false, "if set, the skewer process will be traceable/dumpable")
	serveCmd.Flags().BoolVar(&profile, "profile", false, "if set, profile memory")

	handles = []string{
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

	handlesMap = map[string]int{}
	for i, h := range handles {
		handlesMap[h] = i + 3
	}
}

func runserve() {

	if !dumpableFlag {
		err := dumpable.SetNonDumpable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error setting PR_SET_DUMPABLE: %s\n", err)
		}
	}

	if os.Getenv("SKEWER_LINUX_CHILD") == "TRUE" {
		// we are in the final child on linux
		err := capabilities.NoNewPriv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
		ch := NewServeChild()
		err = ch.Init()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Fatal error in initialization", err)
			os.Exit(-1)
		}
		err = ch.Serve()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Fatal error in Serve():", err)
			os.Exit(-1)
		}
		ch.Cleanup()
		return
	}

	if os.Getenv("SKEWER_CHILD") == "TRUE" {
		// we are in the child
		if capabilities.CapabilitiesSupported {
			// another execve is necessary on Linux to ensure that
			// the following capability drop will be effective on
			// all go threads
			runtime.LockOSThread()
			err := capabilities.DropNetBind()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			exe, err := osext.Executable()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			err = syscall.Exec(exe, os.Args, []string{"PATH=/bin:/usr/bin", "SKEWER_LINUX_CHILD=TRUE"})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
		} else {
			err := capabilities.NoNewPriv()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			ch := NewServeChild()
			err = ch.Init()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Fatal error in initialization", err)
				os.Exit(-1)
			}
			err = ch.Serve()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Fatal error in Serve()", err)
				os.Exit(-1)
			}
			ch.Cleanup()
			return
		}
	}

	// we are in the parent
	if capabilities.CapabilitiesSupported {
		// under Linux, re-exec ourself immediately with fewer privileges
		runtime.LockOSThread()
		need_fix, err := capabilities.NeedFixLinuxPrivileges(uidFlag, gidFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
		if need_fix {
			if os.Getenv("SKEWER_DROPPED") == "TRUE" {
				fmt.Fprintln(os.Stderr, "Dropping privileges failed!")
				fmt.Fprintln(os.Stderr, "Uid", os.Getuid())
				fmt.Fprintln(os.Stderr, "Gid", os.Getgid())
				fmt.Fprintln(os.Stderr, "Capabilities")
				fmt.Fprintln(os.Stderr, capabilities.GetCaps())
				os.Exit(-1)
			}
			err = capabilities.FixLinuxPrivileges(uidFlag, gidFlag)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			err = capabilities.NoNewPriv()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			exe, err := os.Executable()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			err = syscall.Exec(exe, os.Args, []string{"PATH=/bin:/usr/bin", "SKEWER_DROPPED=TRUE"})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
		}
	}

	rootlogger := logging.SetLogging(loglevelFlag, logjsonFlag, syslogFlag, logfilenameFlag)
	logger := rootlogger.New("proc", "parent")

	mustSocketPair := func(typ int) spair {
		a, b, err := sys.SocketPair(typ)
		if err != nil {
			logger.Crit("SocketPair() error", "error", err)
			os.Exit(-1)
		}
		return spair{child: a, parent: b}
	}

	getLoggerConn := func(handle int) net.Conn {
		loggerConn, _ := net.FileConn(os.NewFile(uintptr(handle), "logger"))
		loggerConn.(*net.UnixConn).SetReadBuffer(65536)
		loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
		return loggerConn
	}

	numuid, numgid, err := sys.LookupUid(uidFlag, gidFlag)
	if err != nil {
		logger.Crit("Error looking up uid", "error", err, "uid", uidFlag, "gid", gidFlag)
		os.Exit(-1)
	}
	if numuid == 0 {
		logger.Crit("Provide a non-privileged user with --uid flag")
		os.Exit(-1)
	}

	binderSockets := map[string]spair{}
	loggerSockets := map[string]spair{}

	for _, h := range handles {
		if strings.HasSuffix(h, "_BINDER") {
			binderSockets[h] = mustSocketPair(syscall.SOCK_STREAM)
		} else {
			loggerSockets[h] = mustSocketPair(syscall.SOCK_DGRAM)
		}
	}

	binderParents := []int{}
	for _, s := range binderSockets {
		binderParents = append(binderParents, s.parent)
	}
	err = binder.Binder(binderParents, logger) // returns immediately
	if err != nil {
		logger.Crit("Error setting the root binder", "error", err)
		os.Exit(-1)
	}

	remoteLoggerConn := []net.Conn{}
	for _, s := range loggerSockets {
		remoteLoggerConn = append(remoteLoggerConn, getLoggerConn(s.parent))
	}
	logging.LogReceiver(context.Background(), rootlogger, remoteLoggerConn)

	logger.Debug("Target user", "uid", numuid, "gid", numgid)

	// execute child under the new user
	exe, err := osext.Executable() // custom Executable function to support OpenBSD
	if err != nil {
		logger.Crit("Error getting executable name", "error", err)
		os.Exit(-1)
	}

	extraFiles := []*os.File{}
	for _, h := range handles {
		if strings.HasSuffix(h, "_BINDER") {
			extraFiles = append(extraFiles, os.NewFile(uintptr(binderSockets[h].child), h))
		} else {
			extraFiles = append(extraFiles, os.NewFile(uintptr(loggerSockets[h].child), h))
		}
	}

	childProcess := exec.Cmd{
		Args:       os.Args,
		Path:       exe,
		Stdin:      nil,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		ExtraFiles: extraFiles,
		Env:        []string{"SKEWER_CHILD=TRUE", "PATH=/bin:/usr/bin"},
	}
	if os.Getuid() != numuid {
		childProcess.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(numuid), Gid: uint32(numgid)}}
	}
	err = childProcess.Start()
	if err != nil {
		logger.Crit("Error starting child", "error", err)
		os.Exit(-1)
	}

	for _, h := range handles {
		if strings.HasSuffix(h, "_BINDER") {
			syscall.Close(binderSockets[h].child)
		} else {
			syscall.Close(loggerSockets[h].child)
		}
	}

	sig_chan := make(chan os.Signal, 10)
	once := sync.Once{}
	go func() {
		for sig := range sig_chan {
			logger.Debug("parent received signal", "signal", sig)
			if sig == syscall.SIGTERM {
				once.Do(func() { childProcess.Process.Signal(sig) })
			} else if sig == syscall.SIGHUP {
				childProcess.Process.Signal(sig)
			}
		}
	}()
	signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	logger.Debug("PIDs", "parent", os.Getpid(), "child", childProcess.Process.Pid)

	childProcess.Process.Wait()
	os.Exit(0)

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
	loggerConn, _ := net.FileConn(os.NewFile(uintptr(handlesMap["CHILD_LOGGER"]), "logger"))
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
	return nil
}

func (ch *ServeChild) Cleanup() {
	ch.cancelLogger()
	ch.shutdown()
	ch.globalCancel()
}

func (ch *ServeChild) SetupConfiguration() error {
	ch.confService = services.NewConfigurationService(handlesMap["CONFIG_LOGGER"], ch.logger)
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
	ch.store = services.NewStorePlugin(handlesMap["STORE_LOGGER"], ch.logger)
	ch.store.SetConf(*ch.conf)
	err := ch.store.Create(testFlag, dumpableFlag, storeDirname, "", "")
	if err != nil {
		return fmt.Errorf("Can't create the message Store: %s", err)
	}
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
		binder = handlesMap["RELP_BINDER"]
		logger = handlesMap["RELP_LOGGER"]
	case services.TCP:
		binder = handlesMap["TCP_BINDER"]
		logger = handlesMap["TCP_LOGGER"]
	case services.UDP:
		binder = handlesMap["UDP_BINDER"]
		logger = handlesMap["UDP_LOGGER"]
	case services.Journal:
		logger = handlesMap["JOURNAL_LOGGER"]
	case services.Accounting:
		logger = handlesMap["ACCT_LOGGER"]
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
	return utils.Chain(
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
		err := ch.controllers[services.Accounting].Create(testFlag, dumpableFlag, "", "", ch.conf.Accounting.Path)
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
			err := ctl.Create(testFlag, dumpableFlag, "", "", "")
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
	fmt.Fprintln(os.Stderr, "BOOOOO")
	ctl := ch.controllers[services.RELP]
	err := ctl.Create(testFlag, dumpableFlag, "", "", "")
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
	err := ctl.Create(testFlag, dumpableFlag, "", "", "")
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
	err := ctl.Create(testFlag, dumpableFlag, "", "", "")
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

func (ch *ServeChild) Reload() error {
	ch.logger.Info("Reloading configuration and services")
	// first, let's stop the HTTP server that reports the metrics
	ch.metricsServer.Stop()
	// stop the kafka forwarder
	ch.store.Stop()
	ch.logger.Debug("The forwarder has been stopped")
	ch.store.SetConf(*ch.conf)
	// restart the kafka forwarder
	_, fatal := ch.store.Start()
	if fatal != nil {
		return fatal
	}
	var errJournal error
	var errTCP error
	var errUDP error
	var errAccounting error
	var errRelp error

	wg := &sync.WaitGroup{}

	if journald.Supported {
		// restart the journal service
		wg.Add(1)
		go func() {
			ch.StopController(services.Journal, false)
			errJournal = ch.StartJournal()
			wg.Done()
		}()
	}

	// restart the accounting service
	wg.Add(1)
	go func() {
		ch.StopController(services.Accounting, false)
		errAccounting = ch.StartAccounting()
		wg.Done()
	}()

	// restart the RELP service
	wg.Add(1)
	go func() {
		ch.StopController(services.RELP, false)
		errRelp = ch.StartRelp()
		wg.Done()
	}()

	// restart the TCP service
	wg.Add(1)
	go func() {
		ch.StopController(services.TCP, false)
		errTCP = ch.StartTcp()
		wg.Done()
	}()

	// restart the UDP service
	wg.Add(1)
	go func() {
		ch.StopController(services.UDP, false)
		errUDP = ch.StartUdp()
		wg.Done()
	}()
	wg.Wait()

	// restart the HTTP metrics server
	ch.SetupMetrics()
	return nil
}

func (ch *ServeChild) SetupMetrics() {
	ch.metricsServer = &metrics.MetricsServer{}
	ch.metricsServer.NewConf(
		ch.conf.Metrics,
		ch.controllers[services.Journal],
		ch.controllers[services.RELP],
		ch.controllers[services.TCP],
		ch.controllers[services.UDP],
		ch.controllers[services.Accounting],
		ch.store,
	)
}

func (ch *ServeChild) ShutdownControllers() {
	ch.StopController(services.RELP, true)
	ch.logger.Debug("The RELP service has been stopped")

	ch.StopController(services.Accounting, true)
	ch.logger.Debug("Stopped accounting service")

	ch.StopController(services.Journal, true)
	ch.logger.Debug("Stopped journald service")

	ch.StopController(services.TCP, true)
	ch.logger.Debug("The TCP service has been stopped")

	ch.StopController(services.UDP, true)
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

	defer ch.ShutdownControllers()

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
		fmt.Fprintln(os.Stderr, "main loop")
		select {
		case <-ch.shutdownCtx.Done():
			ch.logger.Info("Shutting down")
			return nil
		default:
		}

		select {
		case <-ch.store.ShutdownChan:
			ch.logger.Crit("Abnormal shutdown of the Store: aborting all operations")
			ch.shutdown()
		default:
		}

		select {
		case <-ch.shutdownCtx.Done():
		case <-ch.store.ShutdownChan:
		case newConf, more := <-ch.confChan:
			if more {
				newConf.Store = ch.conf.Store
				ch.conf = newConf
				err := ch.Reload()
				if err != nil {
					ch.logger.Crit("Fatal error when reloading configuration", "error", err)
					ch.shutdown()
				}
			} else {
				ch.logger.Debug("Configuration channel is closed")
				// newConfChannel has been closed ?!
				select {
				case <-ch.shutdownCtx.Done():
					// this is normal, we are shutting down
				default:
					// not normal
					ch.logger.Crit("Abnormal shutdown of configuration service: aborting all operations")
					ch.shutdown()
				}
			}

		case sig := <-sig_chan:
			if sig == syscall.SIGHUP {
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

			} else if sig == syscall.SIGTERM || sig == syscall.SIGINT {
				signal.Stop(sig_chan)
				signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
				sig_chan = nil
				ch.logger.Info("Termination signal received", "signal", sig)
				ch.shutdown()
			} else {
				ch.logger.Warn("Unknown signal received", "signal", sig)
			}

		}
	}
}
