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
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/dumpable"
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
		err = Serve()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Fatal error in Serve():", err)
			os.Exit(-1)
		}
		os.Exit(0)
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
			err = Serve()
			if err != nil {
				fmt.Fprintln(os.Stderr, "Fatal error in Serve()", err)
				os.Exit(-1)
			}
			os.Exit(0)
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

	handlesToClose := []int{}

	mustSocketPair := func(typ int) (int, int) {
		a, b, err := sys.SocketPair(typ)
		if err != nil {
			logger.Crit("SocketPair() error", "error", err)
			os.Exit(-1)
		}
		handlesToClose = append(handlesToClose, a)
		return a, b
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

	binderChildHandle, binderParentHandle := mustSocketPair(syscall.SOCK_STREAM)
	binderTcpHandle, binderParentTcpHandle := mustSocketPair(syscall.SOCK_STREAM)
	binderUdpHandle, binderParentUdpHandle := mustSocketPair(syscall.SOCK_STREAM)
	binderRelpHandle, binderParentRelpHandle := mustSocketPair(syscall.SOCK_STREAM)

	err = binder.Binder([]int{binderParentHandle, binderParentTcpHandle, binderParentUdpHandle, binderParentRelpHandle}, logger) // returns immediately
	if err != nil {
		logger.Crit("Error setting the root binder", "error", err)
		os.Exit(-1)
	}

	loggerChildHandle, loggerParentHandle := mustSocketPair(syscall.SOCK_DGRAM)
	loggerChildConn := getLoggerConn(loggerParentHandle)

	loggerTcpHandle, loggerParentTcpHandle := mustSocketPair(syscall.SOCK_DGRAM)
	loggerTcpConn := getLoggerConn(loggerParentTcpHandle)

	loggerUdpHandle, loggerParentUdpHandle := mustSocketPair(syscall.SOCK_DGRAM)
	loggerUdpConn := getLoggerConn(loggerParentUdpHandle)

	loggerRelpHandle, loggerParentRelpHandle := mustSocketPair(syscall.SOCK_DGRAM)
	loggerRelpConn := getLoggerConn(loggerParentRelpHandle)

	loggerJournalHandle, loggerParentJournalHandle := mustSocketPair(syscall.SOCK_DGRAM)
	loggerJournalConn := getLoggerConn(loggerParentJournalHandle)

	loggerConfigurationHandle, loggerParentConfigurationHandle := mustSocketPair(syscall.SOCK_DGRAM)
	loggerConfigurationConn := getLoggerConn(loggerParentConfigurationHandle)

	loggerStoreHandle, loggerParentStoreHandle := mustSocketPair(syscall.SOCK_DGRAM)
	loggerStoreConn := getLoggerConn(loggerParentStoreHandle)

	logging.LogReceiver(context.Background(), rootlogger, []net.Conn{
		loggerChildConn, loggerTcpConn, loggerUdpConn, loggerRelpConn, loggerJournalConn, loggerConfigurationConn, loggerStoreConn,
	})

	logger.Debug("Target user", "uid", numuid, "gid", numgid)

	// execute child under the new user
	exe, err := osext.Executable() // custom Executable function to support OpenBSD
	if err != nil {
		logger.Crit("Error getting executable name", "error", err)
		os.Exit(-1)
	}

	childProcess := exec.Cmd{
		Args:   os.Args,
		Path:   exe,
		Stdin:  nil,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		ExtraFiles: []*os.File{
			os.NewFile(uintptr(binderChildHandle), "child_binder_file"),
			os.NewFile(uintptr(binderTcpHandle), "tcp_binder_file"),
			os.NewFile(uintptr(binderUdpHandle), "udp_binder_file"),
			os.NewFile(uintptr(binderRelpHandle), "relp_binder_file"),
			os.NewFile(uintptr(loggerChildHandle), "child_logger_file"),
			os.NewFile(uintptr(loggerTcpHandle), "tcp_logger_file"),
			os.NewFile(uintptr(loggerUdpHandle), "udp_logger_file"),
			os.NewFile(uintptr(loggerRelpHandle), "relp_logger_file"),
			os.NewFile(uintptr(loggerJournalHandle), "journal_logger_file"),
			os.NewFile(uintptr(loggerConfigurationHandle), "config_logger_file"),
			os.NewFile(uintptr(loggerStoreHandle), "store_logger_file"),
		},
		Env: []string{"SKEWER_CHILD=TRUE", "PATH=/bin:/usr/bin"},
	}
	if os.Getuid() != numuid {
		childProcess.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(numuid), Gid: uint32(numgid)}}
	}
	err = childProcess.Start()
	if err != nil {
		logger.Crit("Error starting child", "error", err)
		os.Exit(-1)
	}
	for _, handle := range handlesToClose {
		syscall.Close(handle)
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

func Serve() error {
	globalCtx, gCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdown := context.WithCancel(globalCtx)
	var logger log15.Logger

	binderFile := os.NewFile(3, "binder")

	loggerConn, _ := net.FileConn(os.NewFile(7, "logger"))
	loggerConn.(*net.UnixConn).SetReadBuffer(65536)
	loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
	loggerCtx, cancelLogger := context.WithCancel(context.Background())
	logger = logging.NewRemoteLogger(loggerCtx, loggerConn).New("proc", "child")

	logger.Debug("Serve() runs under user", "uid", os.Getuid(), "gid", os.Getgid())
	if capabilities.CapabilitiesSupported {
		logger.Debug("Capabilities", "caps", capabilities.GetCaps())
	}

	binderClient, err := binder.NewBinderClient(binderFile, logger)
	if err != nil {
		logger.Error("Error binding to the root parent socket", "error", err)
		binderClient = nil
	} else {
		defer binderClient.Quit()
	}

	params := consul.ConnParams{
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

	confSvc := services.NewConfigurationService(12, logger)
	startConfSvc := func() chan *conf.BaseConfig {
		confSvc.SetConfDir(configDirName)
		confSvc.SetConsulParams(params)
		err = confSvc.Start()
		if err != nil {
			logger.Error("Error starting the configuration service", "error", err)
			return nil
		}
		return confSvc.Chan()
	}

	newConfChannel := startConfSvc()
	if newConfChannel == nil {
		time.Sleep(200 * time.Millisecond)
		return fmt.Errorf("Error starting the configuration service")
	}

	c := <-newConfChannel
	c.Store.Dirname = storeDirname
	logger.Info("Store location", "path", c.Store.Dirname)

	// create a consul consulRegistry
	var consulRegistry *consul.Registry
	if consulRegisterFlag {
		consulRegistry, err = consul.NewRegistry(globalCtx, params, consulServiceName, logger)
		if err != nil {
			consulRegistry = nil
		}
	}

	metricsServer := &metrics.MetricsServer{}

	// setup the Store
	st := services.NewStorePlugin(13, logger)
	st.SetConf(*c)
	err = st.Create(testFlag, dumpableFlag, storeDirname, "")
	if err != nil {
		logger.Crit("Can't create the message Store", "error", err)
		time.Sleep(100 * time.Millisecond)
		gCancel()
		cancelLogger()
		return err
	}
	_, err = st.Start()
	if err != nil {
		logger.Crit("Can't start the forwarder", "error", err)
		time.Sleep(100 * time.Millisecond)
		gCancel()
		cancelLogger()
		return err
	}

	sig_chan := make(chan os.Signal, 10)
	signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	relpServicePlugin := services.NewPluginController(services.RELP, st, consulRegistry, 6, 10, logger)
	tcpServicePlugin := services.NewPluginController(services.TCP, st, consulRegistry, 4, 8, logger)
	udpServicePlugin := services.NewPluginController(services.UDP, st, consulRegistry, 5, 9, logger)
	journalServicePlugin := services.NewPluginController(services.Journal, st, consulRegistry, 0, 11, logger)

	startJournal := func(curconf *conf.BaseConfig) {
		// retrieve messages from journald
		if journald.Supported {
			logger.Info("Journald is supported")
			if c.Journald.Enabled {
				logger.Info("Journald service is enabled")
				// in fact Create() will only do something the first time startJournal() is called
				err := journalServicePlugin.Create(testFlag, dumpableFlag, "", "")
				if err != nil {
					logger.Warn("Error creating Journald plugin", "error", err)
					return
				}
				journalServicePlugin.SetConf(*curconf)
				_, err = journalServicePlugin.Start()
				if err != nil {
					logger.Error("Error starting Journald plugin", "error", err)
				} else {
					logger.Debug("Journald plugin has been started")
				}
			} else {
				logger.Info("Journald service is disabled")
			}
		} else {
			logger.Info("Journald service is not supported (only Linux)")
		}
	}

	startRELP := func(curconf *conf.BaseConfig) {
		err := relpServicePlugin.Create(testFlag, dumpableFlag, "", "")
		if err != nil {
			logger.Warn("Error creating RELP plugin", "error", err)
			return
		}
		relpServicePlugin.SetConf(*curconf)
		_, err = relpServicePlugin.Start()
		if err != nil {
			logger.Warn("Error starting RELP plugin", "error", err)
		} else {
			logger.Debug("RELP plugin has been started")
		}
	}

	var tcpinfos []model.ListenerInfo

	startTCP := func(curconf *conf.BaseConfig) {
		err := tcpServicePlugin.Create(testFlag, dumpableFlag, "", "")
		if err != nil {
			logger.Warn("Error creating TCP plugin", "error", err)
			return
		}
		tcpServicePlugin.SetConf(*curconf)
		tcpinfos, err = tcpServicePlugin.Start()
		if err != nil {
			logger.Warn("Error starting TCP plugin", "error", err)
		} else if len(tcpinfos) == 0 {
			logger.Info("TCP plugin not started")
		} else {
			logger.Debug("TCP plugin has been started", "listeners", len(tcpinfos))
		}
	}

	startUDP := func(curconf *conf.BaseConfig) {
		err := udpServicePlugin.Create(testFlag, dumpableFlag, "", "")
		if err != nil {
			logger.Warn("Error creating UDP plugin", "error", err)
			return
		}
		udpServicePlugin.SetConf(*curconf)
		udpinfos, err := udpServicePlugin.Start()
		if err != nil {
			logger.Warn("Error starting UDP plugin", "error", err)
		} else if len(udpinfos) == 0 {
			logger.Info("UDP plugin not started")
		} else {
			logger.Debug("UDP plugin started", "listeners", len(udpinfos))
		}
	}

	stopTCP := func() {
		tcpServicePlugin.Shutdown(3 * time.Second)
	}

	stopUDP := func() {
		udpServicePlugin.Shutdown(3 * time.Second)
	}

	stopRELP := func() {
		relpServicePlugin.Shutdown(3 * time.Second)
	}

	stopJournal := func(shutdown bool) {
		if journald.Supported && c.Journald.Enabled {
			if shutdown {
				journalServicePlugin.Shutdown(5 * time.Second)
			} else {
				// we keep the same instance of the journald plugin, so
				// that we can continue to fetch messages from a
				// consistent position in journald
				journalServicePlugin.Stop()
			}
		}
	}

	Reload := func(newConf *conf.BaseConfig) (fatal error) {
		logger.Info("Reloading configuration")
		// first, let's stop the HTTP server for metrics
		metricsServer.Stop()
		// reset the kafka forwarder
		st.Stop()
		logger.Debug("The forwarder has been stopped")
		st.SetConf(*newConf)
		_, fatal = st.Start()
		if fatal != nil {
			return fatal
		}

		wg := &sync.WaitGroup{}

		if journald.Supported {
			// reset the journald service
			wg.Add(1)
			go func() {
				stopJournal(false)
				startJournal(newConf)
				wg.Done()
			}()
		}

		// reset the RELP service
		wg.Add(1)
		go func() {
			stopRELP()
			startRELP(newConf)
			wg.Done()
		}()

		// reset the TCP service
		wg.Add(1)
		go func() {
			stopTCP()
			startTCP(newConf)
			wg.Done()
		}()

		// reset the UDP service
		wg.Add(1)
		go func() {
			stopUDP()
			startUDP(newConf)
			wg.Done()
		}()
		wg.Wait()

		// restart the HTTP server for metrics
		metricsServer.NewConf(
			newConf.Metrics,
			journalServicePlugin,
			relpServicePlugin,
			tcpServicePlugin,
			udpServicePlugin,
			st,
		)
		return nil
	}

	destructor := func() {
		stopRELP()
		logger.Debug("The RELP service has been stopped")

		stopJournal(true)
		logger.Debug("Stopped journald service")

		stopTCP()
		logger.Debug("The TCP service has been stopped")

		stopUDP()
		logger.Debug("The UDP service has been stopped")

		confSvc.Stop()

		gCancel()
		st.Shutdown(5 * time.Second)
		if consulRegistry != nil {
			consulRegistry.WaitFinished() // wait that the services have been unregistered from Consul
		}
		cancelLogger()
		time.Sleep(time.Second)
	}

	defer destructor()

	startJournal(c)
	startRELP(c)
	startTCP(c)
	startUDP(c)

	metricsServer.NewConf(
		c.Metrics,
		journalServicePlugin,
		relpServicePlugin,
		tcpServicePlugin,
		udpServicePlugin,
		st,
	)

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

	logger.Debug("Main loop is starting")
	for {
		select {
		case <-shutdownCtx.Done():
			logger.Info("Shutting down")
			return nil
		default:
		}

		select {
		case <-st.ShutdownChan:
			logger.Crit("Abnormal shutdown of the Store: aborting all operations")
			shutdown()
		default:
		}

		select {
		case <-shutdownCtx.Done():
		case <-st.ShutdownChan:

		case newConf, more := <-newConfChannel:
			if more {
				newConf.Store = c.Store
				c = newConf
				err := Reload(c)
				if err != nil {
					logger.Crit("Fatal error when reloading configuration", "error", err)
					shutdown()
				}
			} else {
				// newConfChannel has been closed ?!
				select {
				case <-shutdownCtx.Done():
					// this is normal, we are shutting down
				default:
					// not normal, let's try to restart the service
					newConfChannel = startConfSvc()
					if newConfChannel == nil {
						logger.Crit("Can't restart the configuration service: aborting all operations")
						shutdown()
					} else {
						logger.Warn("Configuration service has been restarted")
					}
				}
			}

		case sig := <-sig_chan:
			if sig == syscall.SIGHUP {
				signal.Stop(sig_chan)
				signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
				select {
				case <-shutdownCtx.Done():
				default:
					logger.Info("SIGHUP received: reloading configuration")
					confSvc.Reload()
					sig_chan = make(chan os.Signal, 10)
					signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
				}

			} else if sig == syscall.SIGTERM || sig == syscall.SIGINT {
				signal.Stop(sig_chan)
				signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
				sig_chan = nil
				logger.Info("Termination signal received", "signal", sig)
				shutdown()
			} else {
				logger.Warn("Unknown signal received", "signal", sig)
			}

		}
	}
}
