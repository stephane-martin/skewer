package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
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
	"github.com/stephane-martin/skewer/server"
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

		if os.Getenv("SKEWER_CHILD") == "TRUE" {
			// we are in the child
			sys.NoNewPriv()
			Serve(os.Getenv("SKEWER_HAS_PARENT") == "TRUE", os.Getenv("SKEWER_HAS_ROOT_PARENT") == "TRUE")
			os.Exit(0)
		}

		// we are in the parent
		logfilenameFlag = ""
		rootlogger := utils.SetLogging(loglevelFlag, logjsonFlag, syslogFlag, logfilenameFlag)
		logger := rootlogger.New("proc", "parent")

		numuid, numgid, err := sys.LookupUid(uidFlag, gidFlag)
		if err != nil {
			logger.Crit("Error looking up uid", "error", err, "uid", uidFlag, "gid", gidFlag)
			os.Exit(-1)
		}
		if numuid == 0 {
			logger.Crit("Provide a non-privileged user with --uid flag")
			os.Exit(-1)
		}

		if sys.CapabilitiesSupported {
			// Linux

			need_fix, err := sys.NeedFixLinuxPrivileges(uidFlag, gidFlag)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			}
			if need_fix {
				err = sys.FixLinuxPrivileges(uidFlag, gidFlag) // should not return, but re-exec self
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
				err = syscall.Exec(exe, os.Args, []string{"SKEWER_CHILD=TRUE"})
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				}

			} else {
				sys.NoNewPriv()
				Serve(false, false)
				os.Exit(0)
			}
		} else {
			// not Linux

			binderChildFD, binderParentFD, err := sys.SocketPair(syscall.SOCK_STREAM)
			if err != nil {
				logger.Crit("SocketPair() error", "error", err)
				os.Exit(-1)
			}

			err = sys.Binder(binderParentFD, logger) // returns immediately

			if err != nil {
				logger.Crit("Error setting the root binder", "error", err)
				os.Exit(-1)
			}

			loggerChildFD, loggerParentFD, err := sys.SocketPair(syscall.SOCK_DGRAM)
			if err != nil {
				logger.Crit("SocketPair() error", "error", err)
				os.Exit(-1)
			}

			loggerF := os.NewFile(uintptr(loggerParentFD), "logger")
			loggerConn, _ := net.FileConn(loggerF)
			utils.LogReceiver(context.Background(), loggerConn, rootlogger)

			logger.Debug("Target user", "uid", numuid, "gid", numgid)

			// execute ourself under the new user
			exe, err := sys.Executable() // custom Executable function to support OpenBSD
			if err != nil {
				logger.Crit("Error getting executable name", "error", err)
				os.Exit(-1)
			}

			env := []string{"SKEWER_CHILD=TRUE", "SKEWER_HAS_PARENT=TRUE"}
			if os.Getuid() == 0 {
				env = append(env, "SKEWER_HAS_ROOT_PARENT=TRUE")
			}

			childProcess := exec.Cmd{
				Args:   os.Args,
				Path:   exe,
				Stdin:  nil,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				ExtraFiles: []*os.File{
					os.NewFile(uintptr(binderChildFD), "child_binder_file"),
					os.NewFile(uintptr(loggerChildFD), "child_logger_file"),
				},
				Env: env,
			}
			if os.Getuid() != numuid {
				childProcess.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(numuid), Gid: uint32(numgid)}}
			}
			err = childProcess.Start()
			if err != nil {
				logger.Crit("Error starting child", "error", err)
				os.Exit(-1)
			}
			sig_chan := make(chan os.Signal, 1)
			go func() {
				for sig := range sig_chan {
					logger.Debug("parent received signal (ignored)", "signal", sig)
				}
			}()
			signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
			logger.Debug("PIDs", "parent", os.Getpid(), "child", childProcess.Process.Pid)

			childProcess.Process.Wait()
			os.Exit(0)

		}
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

func Serve(hasParent bool, parentIsRoot bool) error {
	gctx, gCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdown := context.WithCancel(gctx)
	watchCtx, stopWatch := context.WithCancel(shutdownCtx)
	var logger log15.Logger

	if hasParent {
		loggerF := os.NewFile(4, "logger")
		loggerConn, _ := net.FileConn(loggerF)
		logger = utils.NewRemoteLogger(gctx, loggerConn).New("proc", "child")
	} else {
		logger = utils.SetLogging(loglevelFlag, logjsonFlag, syslogFlag, logfilenameFlag).New("proc", "child")
	}

	logger.Debug("Serve() runs under user", "uid", os.Getuid(), "gid", os.Getgid())
	logger.Debug("Whether we have a master process", "master", hasParent)
	logger.Debug("Whether we use a root binder", "root_binder", parentIsRoot)

	var binderClient *sys.BinderClient
	if parentIsRoot {
		var err error
		binderClient, err = sys.NewBinderClient(logger)
		if err != nil {
			logger.Error("Error binding to the root parent socket", "error", err)
			binderClient = nil
		} else {
			defer binderClient.Quit()
		}
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

	var err error
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
		stopForwarder()
		// after stopForwarder() has returned, no more ACK/NACK will be sent to the store,
		// so we can close safely the channels
		// closing the channels makes one of the store goroutine to return
		//close(st.Ack())
		//close(st.Nack())
		//close(st.ProcessingErrors())
		// stop the other Store goroutines (close the inputs channel)
		gCancel()
		// wait that the badger databases are correctly closed
		st.WaitFinished()
		// wait that the services have been unregistered from Consul
		if registry != nil {
			registry.WaitFinished()
		}
		time.Sleep(time.Second)
	}()

	// retrieve linux audit logs

	var auditService *server.AuditService
	var auditCtx context.Context
	var cancelAudit context.CancelFunc
	if c.Audit.Enabled && auditlogs.Supported {
		if !sys.CanReadAuditLogs() {
			logger.Info("Audit logs are requested, but the needed Linux Capability is not present. Disabling.")
		} else {
			logger.Info("Linux audit logs are enabled")
			auditCtx, cancelAudit = context.WithCancel(shutdownCtx)
			auditService = server.NewAuditService(st, generator, metricStore, logger)
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

	// retrieve messages from journald
	var journaldServer *server.JournaldServer
	if c.Journald.Enabled && journald.Supported {
		logger.Info("Journald support is enabled")
		journalCtx, cancelJournal := context.WithCancel(gctx)
		journaldServer, err = server.NewJournaldServer(journalCtx, c.Journald, st, generator, metricStore, logger)
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
		logger.Info("Journald support is disabled (not requested or not Linux)")
	}

	// prepare the RELP service
	relpServer := server.NewRelpServer(c, metricStore, logger)
	if testFlag {
		relpServer.SetTest()
	}
	sig_chan := make(chan os.Signal)
	signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	relpServer.StatusChan <- server.Stopped // trigger the RELP service to start

	// start the TCP service
	tcpServer := server.NewTcpServer(c, st, generator, binderClient, metricStore, logger)
	if testFlag {
		tcpServer.SetTest()
	}
	err = tcpServer.Start()
	if err == nil {
		tcpServer.Register(registry)
	} else {
		logger.Error("Error starting the TCP server", "error", err)
	}

	// start the UDP service
	udpServer := server.NewUdpServer(c, st, generator, binderClient, metricStore, logger)
	if testFlag {
		udpServer.SetTest()
	}
	err = udpServer.Start()
	if err != nil {
		logger.Error("Error starting the UDP server", "error", err)
	}

	Reload := func(newConf *conf.GConfig) {
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
			relpServer.Unregister(registry)
			relpServer.Stop()
			wg.Done()
		}()

		// reset the TCP service
		wg.Add(1)
		go func() {
			tcpServer.Unregister(registry)
			tcpServer.Stop()
			<-tcpServer.ClosedChan
			tcpServer.Conf = *newConf
			err := tcpServer.Start()
			if err == nil {
				tcpServer.Register(registry)
			} else {
				logger.Error("Error starting the TCP server", "error", err)
			}
			wg.Done()
		}()

		// reset the UDP service
		wg.Add(1)
		go func() {
			udpServer.Stop()
			<-udpServer.ClosedChan
			udpServer.Conf = *newConf
			err := udpServer.Start()
			if err != nil {
				logger.Error("Error starting the UDP server", "error", err)
			}
			wg.Done()
		}()
		wg.Wait()
	}

	logger.Debug("Main loop is starting")
	for {
		select {
		case <-shutdownCtx.Done():
			logger.Info("Shutting down")
			relpServer.Unregister(registry)
			relpServer.FinalStop()
			_, more := <-relpServer.StatusChan
			for more {
				_, more = <-relpServer.StatusChan
			}
			logger.Debug("The RELP service has been stopped")

			if journaldServer != nil {
				journaldServer.Stop()
			}
			if auditService != nil {
				cancelAudit()
				auditService.WaitFinished()
			}

			tcpServer.Unregister(registry)
			tcpServer.Stop()
			udpServer.Stop()
			<-tcpServer.ClosedChan
			logger.Debug("The TCP service has been stopped")
			<-udpServer.ClosedChan
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

		case state := <-relpServer.StatusChan:
			switch state {
			case server.FinalStopped:
				logger.Debug("The RELP service has been definitely halted")

			case server.Stopped:
				logger.Debug("The RELP service is stopped")
				relpServer.Conf = *c
				err := relpServer.Start()
				if err != nil {
					logger.Warn("The RELP service has failed to start", "error", err)
					relpServer.StopAndWait()
				}

			case server.Waiting:
				logger.Debug("RELP waiting")
				go func() {
					time.Sleep(time.Duration(30) * time.Second)
					relpServer.EndWait()
				}()

			case server.Started:
				logger.Debug("The RELP service has been started")
				relpServer.Register(registry)
			}

		}

	}
}
