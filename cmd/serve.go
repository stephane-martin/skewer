package cmd

import (
	"context"
	"fmt"
	"log/syslog"
	"os"
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

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start listening for Syslog messages and forward them to Kafka",
	Long: `The serve command is the main skewer command. It launches a long
running process that listens to syslog messages according to the configuration,
connects to Kafka, and forwards messages to Kafka.`,
	Run: func(cmd *cobra.Command, args []string) {
		if sys.CapabilitiesSupported {
			err := sys.FixLinuxPrivileges(uidFlag, gidFlag)
			if err != nil {
				fmt.Println(err)
				os.Exit(-1)
			}
		}
		Serve()
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

func init() {
	RootCmd.AddCommand(serveCmd)
	serveCmd.Flags().BoolVar(&testFlag, "test", false, "Print to stdout instead of sending to Kafka")
	serveCmd.Flags().BoolVar(&syslogFlag, "syslog", false, "Send logs to the local syslog")
	serveCmd.Flags().StringVar(&loglevelFlag, "loglevel", "info", "Set logging level")
	serveCmd.Flags().StringVar(&logfilenameFlag, "logfilename", "", "Write logs to a file instead of stderr")
	serveCmd.Flags().BoolVar(&logjsonFlag, "json", false, "Write logs in JSON format")
	serveCmd.Flags().StringVar(&pidFilenameFlag, "pidfile", "", "If given, write PID to file")
	serveCmd.Flags().BoolVar(&registerFlag, "register", false, "Register services in consul")
	serveCmd.Flags().StringVar(&serviceName, "servicename", "skewer", "Service name to register in consul")
	serveCmd.Flags().StringVar(&uidFlag, "uid", "", "Switch to this user ID")
	serveCmd.Flags().StringVar(&gidFlag, "gid", "", "Switch to this group ID")
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
			os.Exit(-1)
		}
		log_handlers = append(log_handlers, h)
	}
	logfilenameFlag = strings.TrimSpace(logfilenameFlag)
	if len(logfilenameFlag) > 0 {
		h, e := log15.FileHandler(logfilenameFlag, formatter)
		if e != nil {
			fmt.Printf("Error opening log file '%s': %s\n", logfilenameFlag, e)
			os.Exit(-1)
		}
		log_handlers = append(log_handlers, h)
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

func Serve() {
	gctx, gCancel := context.WithCancel(context.Background())
	shutdownCtx, shutdown := context.WithCancel(gctx)
	watchCtx, stopWatch := context.WithCancel(shutdownCtx)

	logger := SetLogging()
	generator := utils.Generator(gctx, logger)

	var err error
	err = sys.SetNonDumpable()
	if err != nil {
		logger.Info("Error setting PR_SET_DUMPABLE", "error", err)
	}

	err = sys.MlockAll()
	if err != nil {
		logger.Info("Error executing MlockAll()", "error", err)
	}

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
		c, updated, err = conf.InitLoad(watchCtx, configDirName, params, consulPrefix, logger)
		if err == nil {
			break
		}
		logger.Error("Error getting configuration. Sleep and retry.", "error", err)
		time.Sleep(30 * time.Second)
	}

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
		os.Exit(-1)
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
		logger.Info("Linux audit logs are disabled (not requested)")
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
		logger.Info("Journald support is disabled")
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
	tcpServer := server.NewTcpServer(c, st, generator, metricStore, logger)
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
	udpServer := server.NewUdpServer(c, st, generator, metricStore, logger)
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
			logger.Info("The TCP service has been stopped")
			<-udpServer.ClosedChan
			logger.Info("The UDP service has been stopped")
			return

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
				logger.Info("The RELP service has been definitely halted")

			case server.Stopped:
				logger.Info("The RELP service has been stopped")
				relpServer.Conf = *c
				err := relpServer.Start()
				if err != nil {
					logger.Warn("The RELP service has failed to start", "error", err)
					relpServer.StopAndWait()
				}

			case server.Waiting:
				logger.Info("Waiting")
				go func() {
					time.Sleep(time.Duration(30) * time.Second)
					relpServer.EndWait()
				}()

			case server.Started:
				logger.Info("The RELP service has been started")
				relpServer.Register(registry)
			}

		}

	}
}
