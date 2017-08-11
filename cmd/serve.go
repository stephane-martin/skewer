package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/skewer/auditlogs"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/consul"
	"github.com/stephane-martin/skewer/journald"
	"github.com/stephane-martin/skewer/metrics"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/store"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start listening for Syslog messages and forward them to Kafka",
	Long: `The serve command is the main skewer command. It launches a long
running process that listens to syslog messages according to the configuration,
connects to Kafka, and forwards messages to Kafka.`,
	Run: func(cmd *cobra.Command, args []string) {

		// try to set mlock and non-dumpable for both child and parent
		if sys.MlockSupported && !noMlockFlag {
			err := sys.MlockAll()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error executing MlockAll(): %s\n", err)
			}
		}

		if sys.CapabilitiesSupported && !dumpableFlag {
			err := sys.SetNonDumpable()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting PR_SET_DUMPABLE: %s\n", err)
			}
		}

		if os.Getenv("SKEWER_LINUX_CHILD") == "TRUE" {
			// we are in the final child on linux
			sys.NoNewPriv()
			Serve()
			os.Exit(0)
		}

		if os.Getenv("SKEWER_CHILD") == "TRUE" {
			// we are in the child
			if sys.CapabilitiesSupported {
				// another execve is necessary on Linux to ensure that
				// the following capability drop will be effective on
				// all go threads
				runtime.LockOSThread()
				sys.DropNetBind()
				exe, err := os.Executable()
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
				sys.NoNewPriv()
				Serve()
				os.Exit(0)
			}
		}

		// we are in the parent

		if sys.CapabilitiesSupported {
			// under Linux, re-exec ourself immediately with fewer privileges
			runtime.LockOSThread()
			need_fix, err := sys.NeedFixLinuxPrivileges(uidFlag, gidFlag)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			if need_fix {
				if os.Getenv("SKEWER_DROPPED") == "TRUE" {
					fmt.Fprintln(os.Stderr, "Dropping privileges failed!")
					os.Exit(-1)
				}
				err = sys.FixLinuxPrivileges(uidFlag, gidFlag)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				}
				sys.NoNewPriv()
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

		rootlogger := utils.SetLogging(loglevelFlag, logjsonFlag, syslogFlag, logfilenameFlag)
		logger := rootlogger.New("proc", "parent")

		mustSocketPair := func(typ int) (int, int) {
			a, b, err := sys.SocketPair(typ)
			if err != nil {
				logger.Crit("SocketPair() error", "error", err)
				os.Exit(-1)
			}
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

		err = sys.Binder([]int{binderParentHandle, binderParentTcpHandle, binderParentUdpHandle, binderParentRelpHandle}, logger) // returns immediately
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

		utils.LogReceiver(context.Background(), rootlogger, []net.Conn{loggerChildConn, loggerTcpConn, loggerUdpConn, loggerRelpConn})

		logger.Debug("Target user", "uid", numuid, "gid", numgid)

		// execute child under the new user
		exe, err := sys.Executable() // custom Executable function to support OpenBSD
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
		syscall.Close(binderChildHandle)
		syscall.Close(binderTcpHandle)
		syscall.Close(binderUdpHandle)
		syscall.Close(binderRelpHandle)
		syscall.Close(loggerChildHandle)
		syscall.Close(loggerTcpHandle)
		syscall.Close(loggerUdpHandle)
		syscall.Close(loggerRelpHandle)

		sig_chan := make(chan os.Signal, 1)
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

	},
}

var testFlag bool
var syslogFlag bool
var loglevelFlag string
var logfilenameFlag string
var logjsonFlag bool
var pidFilenameFlag string
var registerFlag bool
var serviceName string
var uidFlag string
var gidFlag string
var dumpableFlag bool
var noMlockFlag bool

func init() {
	RootCmd.AddCommand(serveCmd)
	serveCmd.Flags().BoolVar(&testFlag, "test", false, "Print messages to stdout instead of sending to Kafka")
	serveCmd.Flags().BoolVar(&syslogFlag, "syslog", false, "Send logs to the local syslog (are you sure you wan't to do that ?)")
	serveCmd.Flags().StringVar(&loglevelFlag, "loglevel", "info", "Set logging level")
	serveCmd.Flags().StringVar(&logfilenameFlag, "logfilename", "", "Write logs to a file instead of stderr")
	serveCmd.Flags().BoolVar(&logjsonFlag, "logjson", false, "Write logs in JSON format")
	serveCmd.Flags().StringVar(&pidFilenameFlag, "pidfile", "", "If given, write PID to file")
	serveCmd.Flags().BoolVar(&registerFlag, "register", false, "Register services in consul")
	serveCmd.Flags().StringVar(&serviceName, "servicename", "skewer", "Service name to register in consul")
	serveCmd.Flags().StringVar(&uidFlag, "uid", "", "Switch to this user ID (when launched as root)")
	serveCmd.Flags().StringVar(&gidFlag, "gid", "", "Switch to this group ID (when launched as root)")
	serveCmd.Flags().BoolVar(&dumpableFlag, "dumpable", false, "if set, the skewer process will be traceable/dumpable")
	serveCmd.Flags().BoolVar(&noMlockFlag, "no-mlock", false, "if set, skewer will not mlock() its memory")
}

func Serve() error {
	gctx, gCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdown := context.WithCancel(gctx)
	watchCtx, stopWatch := context.WithCancel(shutdownCtx)
	var logger log15.Logger

	binderFile := os.NewFile(3, "binder")

	loggerConn, _ := net.FileConn(os.NewFile(7, "logger"))
	loggerConn.(*net.UnixConn).SetReadBuffer(65536)
	loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
	loggerCtx, cancelLogger := context.WithCancel(context.Background())
	logger = utils.NewRemoteLogger(loggerCtx, loggerConn).New("proc", "child")

	logger.Debug("Serve() runs under user", "uid", os.Getuid(), "gid", os.Getgid())
	if sys.CapabilitiesSupported {
		logger.Debug("Capabilities", "caps", sys.GetCaps())
	}

	binderClient, err := sys.NewBinderClient(binderFile, logger)
	if err != nil {
		logger.Error("Error binding to the root parent socket", "error", err)
		binderClient = nil
	} else {
		defer binderClient.Quit()
	}

	generator := utils.Generator(gctx, logger)

	var c *conf.GConfig
	var st store.Store
	var updated chan bool

	params := consul.ConnParams{
		Address:    consulAddr,
		Datacenter: consulDC,
		Token:      consulToken,
		CAFile:     consulCAFile,
		CAPath:     consulCAPath,
		CertFile:   consulCertFile,
		KeyFile:    consulKeyFile,
		Insecure:   consulInsecure,
	}

	// read configuration
	for {
		c, updated, err = conf.InitLoad(watchCtx, configDirName, storeDirname, consulPrefix, params, logger)
		if err == nil {
			break
		}
		logger.Error("Error getting configuration. Sleep and retry.", "error", err)
		time.Sleep(30 * time.Second)
	}
	logger.Info("Store location", "path", c.Store.Dirname)

	// create a consul registry
	var registry *consul.Registry
	if registerFlag {
		registry, err = consul.NewRegistry(gctx, params, serviceName, logger)
		if err != nil {
			registry = nil
		}
	}

	metricStore := metrics.SetupMetrics(c.Metrics)

	// prepare the message store
	st, err = store.NewStore(gctx, c.Store, metricStore, logger)
	if err != nil {
		logger.Crit("Can't create the message Store", "error", err)
		gCancel()
		cancelLogger()
		return err
	}
	err = st.StoreAllSyslogConfigs(c)
	if err != nil {
		logger.Crit("Can't store the syslog configurations", "error", err)
		gCancel()
		cancelLogger()
		return err
	}

	// prepare the kafka forwarder
	forwarder := store.NewForwarder(testFlag, metricStore, logger)
	forwarderMutex := &sync.Mutex{}
	var cancelForwarder context.CancelFunc

	startForwarder := func(kafkaConf conf.KafkaConfig) {
		forwarderMutex.Lock()
		defer forwarderMutex.Unlock()
		newForwarderCtx, newCancelForwarder := context.WithCancel(shutdownCtx)
		if forwarder.Forward(newForwarderCtx, st, kafkaConf) {
			cancelForwarder = newCancelForwarder
		}
	}
	stopForwarder := func() {
		forwarderMutex.Lock()
		defer forwarderMutex.Unlock()
		cancelForwarder()
		forwarder.WaitFinished()
	}

	startForwarder(c.Kafka)

	defer func() {
		// wait that the forwarder has been closed to shutdown the store
		stopForwarder()   // after stopForwarder() has returned, no more ACK/NACK will be sent to the store
		gCancel()         // stop the Store goroutines (close the inputs channel)
		st.WaitFinished() // wait that the badger databases are correctly closed
		if registry != nil {
			registry.WaitFinished() // wait that the services have been unregistered from Consul
		}
		cancelLogger()
		time.Sleep(time.Second)
	}()

	// retrieve linux audit logs

	var auditService *services.AuditService
	var auditCtx context.Context
	var cancelAudit context.CancelFunc
	if auditlogs.Supported {
		logger.Info("Linux audit logs are supported")
		if c.Audit.Enabled {
			if !sys.CanReadAuditLogs() {
				logger.Info("Audit logs are requested, but the needed Linux Capability is not present. Disabling.")
			} else if sys.HasAnyProcess([]string{"go-audit", "auditd"}) {
				logger.Warn("Audit logs are requested, but go-audit or auditd process is already running, so we disable audit logs")
			} else {
				logger.Info("Linux audit logs are enabled")
				auditCtx, cancelAudit = context.WithCancel(shutdownCtx)
				auditService = services.NewAuditService(st, generator, metricStore, logger)
				err := auditService.Start(auditCtx, c.Audit)
				if err != nil {
					logger.Warn("Error starting the linux audit service", "error", err)
					cancelAudit()
					auditService = nil
				} else {
					logger.Debug("Linux audit logs service is started")
				}

			}
		} else {
			logger.Info("Linux audit logs are disabled (not requested or not Linux)")
		}
	} else {
		logger.Info("Linux audit logs are not supported")
	}

	// retrieve messages from journald
	var journaldServer *services.JournaldServer
	if journald.Supported {
		logger.Info("Journald is supported")
		if c.Journald.Enabled {
			logger.Info("Journald service is enabled")

			journalCtx, cancelJournal := context.WithCancel(gctx)
			journaldServer, err = services.NewJournaldServer(journalCtx, c.Journald, st, generator, metricStore, logger)
			if err == nil {
				err = journaldServer.Start()
				if err != nil {
					logger.Warn("Error starting the journald service", "error", err)
					cancelJournal()
					journaldServer = nil
				} else {
					logger.Debug("Journald service is started")
				}
			} else {
				logger.Warn("Error initializing the journald service", "error", err)
				journaldServer = nil
			}
		} else {
			logger.Info("Journald service is disabled")
		}
	} else {
		logger.Info("Journald not supported")
	}

	sig_chan := make(chan os.Signal)
	signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	var relpServicePlugin services.NetworkService
	var tcpServicePlugin services.NetworkService
	var udpServicePlugin services.NetworkService

	startRELP := func(curconf *conf.GConfig) {
		var err error
		relpServicePlugin = services.NewNetworkPlugin("relp", st, 6, 10, metricStore, logger)
		relpServicePlugin.SetConf(curconf.Syslog, curconf.Parsers)
		relpServicePlugin.SetKafkaConf(&curconf.Kafka)
		_, err = relpServicePlugin.Start(testFlag)
		if err != nil {
			logger.Error("Error starting RELP plugin", "error", err)
		} else {
			logger.Debug("RELP plugin has been started")
		}
	}

	var tcpinfos []*model.ListenerInfo

	startTCP := func(curconf *conf.GConfig) {
		var err error
		tcpServicePlugin = services.NewNetworkPlugin("tcp", st, 4, 8, metricStore, logger)
		tcpServicePlugin.SetConf(curconf.Syslog, curconf.Parsers)
		tcpServicePlugin.SetKafkaConf(&curconf.Kafka)
		tcpinfos, err = tcpServicePlugin.Start(testFlag)
		if err != nil {
			logger.Error("Error starting TCP plugin", "error", err)
		} else if len(tcpinfos) == 0 {
			logger.Info("TCP plugin not started")
		} else {
			logger.Debug("TCP plugin has been started", "listeners", len(tcpinfos))
			if registry != nil {
				for _, infos := range tcpinfos {
					registry.RegisterTcpListener(infos)
				}
			}
		}
	}

	startUDP := func(curconf *conf.GConfig) {
		var err error
		udpServicePlugin = services.NewNetworkPlugin("udp", st, 5, 9, metricStore, logger)
		udpServicePlugin.SetConf(curconf.Syslog, curconf.Parsers)
		udpServicePlugin.SetKafkaConf(&curconf.Kafka)
		udpinfos, err := udpServicePlugin.Start(testFlag)
		if err != nil {
			logger.Error("Error starting UDP plugin", "error", err)
		} else if len(udpinfos) == 0 {
			logger.Info("UDP plugin not started")
		} else {
			logger.Debug("UDP plugin started", "listeners", len(udpinfos))
		}
	}

	startRELP(c)
	startTCP(c)
	startUDP(c)

	stopTCP := func() {
		tcpServicePlugin.Stop()
		tcpServicePlugin.WaitClosed()
		if len(tcpinfos) > 0 && registry != nil {
			for _, infos := range tcpinfos {
				registry.UnregisterTcpListener(infos)
			}
			tcpinfos = nil
		}
	}

	stopUDP := func() {
		udpServicePlugin.Stop()
		udpServicePlugin.WaitClosed()
	}

	stopRELP := func() {
		relpServicePlugin.Stop()
		relpServicePlugin.WaitClosed()
	}

	Reload := func(newConf *conf.GConfig) {
		err := st.StoreAllSyslogConfigs(newConf)
		if err != nil {
			logger.Crit("Can't store the syslog configurations", "error", err)
		}

		metricStore.NewConf(newConf.Metrics)
		// reset the kafka forwarder
		stopForwarder()
		startForwarder(newConf.Kafka)

		wg := &sync.WaitGroup{}

		// reset the journald service
		if journaldServer != nil {
			wg.Add(1)
			go func() {
				journaldServer.Stop()
				journaldServer.Conf = newConf.Journald
				journaldServer.Start()
				wg.Done()
			}()
		}

		// reset the audit service
		if auditService != nil {
			wg.Add(1)
			go func() {
				cancelAudit()
				auditService.WaitFinished()
				auditCtx, cancelAudit = context.WithCancel(shutdownCtx)
				err := auditService.Start(auditCtx, newConf.Audit)
				if err != nil {
					cancelAudit()
					auditService = nil
					logger.Warn("Error starting the linux audit service", "error", err)
				}
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
	}

	logger.Debug("Main loop is starting")
	for {
		select {
		case <-shutdownCtx.Done():
			logger.Info("Shutting down")

			//relpServer.Unregister(registry)
			//relpServer.FinalStop()
			//_, more := <-relpServer.StatusChan
			//for more {
			//	_, more = <-relpServer.StatusChan
			//}
			stopRELP()
			logger.Debug("The RELP service has been stopped")

			if journaldServer != nil {
				logger.Debug("Stopping journald service")
				journaldServer.Stop()
				logger.Debug("Stopped journald service")
			}

			if auditService != nil {
				logger.Debug("Stopping audit service")
				cancelAudit()
				auditService.WaitFinished()
				logger.Debug("Stopped audit service")
			}

			logger.Debug("Stopping TCP and UDP services")
			stopTCP()
			logger.Debug("The TCP service has been stopped")
			stopUDP()
			logger.Debug("The UDP service has been stopped")

			return nil

		case _, more := <-updated:
			if more {
				select {
				case <-shutdownCtx.Done():
				default:
					logger.Info("Configuration was updated by Consul")
					Reload(c)
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
					newWatchCtx, newStopWatch := context.WithCancel(shutdownCtx)
					newConf, newUpdated, err := c.Reload(newWatchCtx) // try to reload the configuration
					if err == nil {
						stopWatch() // stop watch the old config
						stopWatch = newStopWatch
						updated = newUpdated
						Reload(newConf)
						*c = *newConf
					} else {
						newStopWatch()
						logger.Error("Error reloading configuration. Configuration was left untouched.", "error", err)
					}
					sig_chan = make(chan os.Signal)
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

		case <-st.Errors():
			logger.Warn("The store had a fatal error")
			shutdown()

		case <-forwarder.ErrorChan():
			logger.Warn("Forwarder has received a fatal Kafka error: resetting connection to Kafka")
			kafkaConf := c.Kafka
			stopForwarder()
			go func() {
				time.Sleep(time.Second)
				startForwarder(kafkaConf)
			}()

		}

	}
}
