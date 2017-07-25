package cmd

import (
	"context"
	"fmt"
	"log/syslog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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

		if !dumpableFlag {
			err := sys.SetNonDumpable()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error setting PR_SET_DUMPABLE: %s\n", err)
			}
		}

		if !noMlockFlag {
			err := sys.MlockAll()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error executing MlockAll(): %s\n", err)
			}
		}

		if sys.CapabilitiesSupported {
			// Linux
			fix, err := sys.NeedFixLinuxPrivileges(uidFlag, gidFlag)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
			} else if fix {
				err = sys.FixLinuxPrivileges(uidFlag, gidFlag) // should not return, but re-exec self
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				} else {
					os.Exit(0)
				}
			} else {
				// we have the appropriate privileges
				Serve(false)
				os.Exit(0)
			}
		} else {
			// not Linux
			if os.Getuid() != 0 {
				fmt.Fprintf(os.Stderr, "cur uid: %d, cur gid: %d\n", os.Getuid(), os.Getgid())
				Serve(os.Getenv("SKEWER_ROOT_PARENT") == "TRUE")
				fmt.Fprintln(os.Stderr, "end of child")
				os.Exit(0)

			} else {
				numuid, numgid, err := sys.LookupUid(uidFlag, gidFlag)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				}
				if numuid == 0 {
					fmt.Fprintln(os.Stderr, "Provide a non-privileged user with --uid flag")
					os.Exit(-1)
				}
				childFD, parentFD, err := sys.SocketPair()
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				}

				fmt.Fprintf(os.Stderr, "target uid: %d, target gid: %d\n", numuid, numgid)

				// execute ourself under the new user
				exe, err := sys.Executable()
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				}
				childProcess := exec.Cmd{
					Args:       os.Args,
					Path:       exe,
					Stdin:      nil,
					Stdout:     os.Stdout,
					Stderr:     os.Stderr,
					ExtraFiles: []*os.File{os.NewFile(uintptr(childFD), "child_file")},
					Env:        []string{"SKEWER_ROOT_PARENT=TRUE"},
				}
				if os.Getuid() != numuid {
					childProcess.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(numuid), Gid: uint32(numgid)}}
				}
				err = childProcess.Start()
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				}
				sig_chan := make(chan os.Signal, 1)
				go func() {
					for sig := range sig_chan {
						fmt.Fprintln(os.Stderr, "parent received signal", sig)
					}
				}()
				signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
				//signal.Ignore(syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT) // so that signals only notify the child
				fmt.Fprintf(os.Stderr, "Parent PID: %d, Child PID: %d\n", os.Getpid(), childProcess.Process.Pid)

				err = sys.Binder(parentFD)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				}
				childProcess.Process.Wait()
				fmt.Fprintln(os.Stderr, "end of parent")
				os.Exit(0)

			}

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

func SetLogging() log15.Logger {
	logger := log15.New()
	log_handlers := []log15.Handler{}
	var formatter log15.Format
	if logjsonFlag {
		formatter = log15.JsonFormat()
	} else {
		formatter = log15.LogfmtFormat()
	}
	if syslogFlag {
		h, e := log15.SyslogHandler(syslog.LOG_LOCAL0|syslog.LOG_DEBUG, "skewer", formatter)
		if e != nil {
			fmt.Printf("Error opening syslog file: %s\n", e)
		} else {
			log_handlers = append(log_handlers, h)
		}
	}
	logfilenameFlag = strings.TrimSpace(logfilenameFlag)
	if len(logfilenameFlag) > 0 {
		h, e := log15.FileHandler(logfilenameFlag, formatter)
		if e != nil {
			fmt.Printf("Error opening log file '%s': %s\n", logfilenameFlag, e)
		} else {
			log_handlers = append(log_handlers, h)
		}
	}
	if len(log_handlers) == 0 {
		log_handlers = []log15.Handler{log15.StderrHandler}
	}
	handler := log15.MultiHandler(log_handlers...)

	lvl, e := log15.LvlFromString(loglevelFlag)
	if e != nil {
		lvl = log15.LvlInfo
	}
	handler = log15.LvlFilterHandler(lvl, handler)

	logger.SetHandler(handler)
	return logger
}

func Serve(hasBinder bool) error {
	var binderClient *sys.BinderClient
	if hasBinder {
		var err error
		binderClient, err = sys.NewBinderClient()
		if err != nil {
			binderClient = nil
		} else {
			defer binderClient.Quit()
		}
	}

	gctx, gCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdown := context.WithCancel(gctx)
	watchCtx, stopWatch := context.WithCancel(shutdownCtx)

	logger := SetLogging()
	logger.Debug("using root binder", "use", hasBinder)
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

	metricStore := metrics.SetupMetrics()

	// prepare the message store
	st, err = store.NewStore(gctx, c.Store, logger)
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
		close(st.Ack())
		close(st.Nack())
		close(st.ProcessingErrors())
		// stop the other Store goroutines
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
			if journaldServer != nil {
				journaldServer.Stop()
			}
			relpServer.Unregister(registry)
			relpServer.FinalStop()
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
