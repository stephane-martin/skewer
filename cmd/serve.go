package cmd

import (
	"fmt"
	"log/syslog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/spf13/cobra"
	"github.com/stephane-martin/relp2kafka/conf"
	"github.com/stephane-martin/relp2kafka/server"
)

var logger = log15.New()

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		Serve()
	},
}

var testFlag bool
var syslogFlag bool
var loglevelFlag string
var logfilenameFlag string
var logjsonFlag bool
var pidFilenameFlag string

func init() {
	RootCmd.AddCommand(serveCmd)
	serveCmd.Flags().BoolVar(&testFlag, "test", false, "Print to stdout instead of sending to Kafka")
	serveCmd.Flags().BoolVar(&syslogFlag, "syslog", false, "Send logs to the local syslog")
	serveCmd.Flags().StringVar(&loglevelFlag, "loglevel", "info", "Set logging level")
	serveCmd.Flags().StringVar(&logfilenameFlag, "logfilename", "", "Write logs to a file instead of stderr")
	serveCmd.Flags().BoolVar(&logjsonFlag, "json", false, "Write logs in JSON format")
	serveCmd.Flags().StringVar(&pidFilenameFlag, "pidfile", "", "If given, write PID to file")

}

func setLogging() {
	log_handlers := []log15.Handler{}
	var formatter log15.Format
	if logjsonFlag {
		formatter = log15.JsonFormat()
	} else {
		formatter = log15.LogfmtFormat()
	}
	if syslogFlag {
		h, e := log15.SyslogHandler(syslog.LOG_LOCAL0|syslog.LOG_DEBUG, "relp2kafka", formatter)
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
}

func Serve() {
	setLogging()
	c, e := conf.Load(configDirName)
	if e != nil {
		fmt.Println(e)
		os.Exit(-1)
	}

	relpServer := server.NewRelpServer(c, logger)
	if testFlag {
		relpServer.SetTest()
	}
	sig_chan := make(chan os.Signal)
	signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	relpServer.StatusChan <- server.Stopped

	tcpServer := server.NewTcpServer(c, logger)
	if testFlag {
		tcpServer.SetTest()
	}
	e = tcpServer.Start()
	if e != nil {
		logger.Error("Error starting the TCP server", "error", e)
	} else {
		logger.Info("The TCP service has started")
	}

	go func() {
		for {
			select {
			case sig := <-sig_chan:
				if sig == syscall.SIGHUP {
					logger.Info("SIGHUP received")
					relpServer.Stop()
					tcpServer.Stop()
					<-tcpServer.StatusChan
					tcpServer.Start()

				} else if sig == syscall.SIGTERM || sig == syscall.SIGINT {
					logger.Info("Termination signal received", "signal", sig)
					relpServer.FinalStop()
					tcpServer.Stop()
				}
			}
		}
	}()

	for {
		state := <-relpServer.StatusChan
		switch state {
		case server.FinalStopped:
			logger.Info("The RELP service was definitely halted")
			// wait that the tcp service stops too
			<-tcpServer.StatusChan
			logger.Info("The TCP service has stopped")
			return
		case server.Stopped:
			err := relpServer.Start()
			if err != nil {
				logger.Warn("The RELP service failed to start", "error", err)
				relpServer.StopAndWait()
			}
		case server.Waiting:
			logger.Info("Waiting")
			go func() {
				time.Sleep(time.Duration(30) * time.Second)
				relpServer.EndWait()
			}()

		case server.Started:
			logger.Info("The RELP service has started")
		}
	}
}
