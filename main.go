package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/cmd"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

func main() {

	getLogger := func(ctx context.Context, name string, handle int) log15.Logger {
		loggerConn, err := net.FileConn(os.NewFile(uintptr(handle), "logger"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil
		}
		err = loggerConn.(*net.UnixConn).SetReadBuffer(65536)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		err = loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return utils.NewRemoteLogger(ctx, loggerConn).New("proc", name)
	}

	switch name := os.Args[0]; name {
	case "skewer-tcp", "skewer-udp", "skewer-relp", "skewer-journal", "skewer-audit":
		var binderClient *sys.BinderClient
		var err error
		loggerCtx, cancelLogger := context.WithCancel(context.Background())
		logger := log15.New()
		if os.Getenv("HAS_BINDER") == "TRUE" {
			if os.Getenv("HAS_LOGGER") == "TRUE" {
				logger = getLogger(loggerCtx, name, 4)
			}
			binderClient, _ = sys.NewBinderClient(os.NewFile(3, "binder"), logger)
		} else if os.Getenv("HAS_LOGGER") == "TRUE" {
			logger = getLogger(loggerCtx, name, 3)
		}
		if logger == nil {
			fmt.Fprintln(os.Stderr, "Could not create a logger for the plugin")
			cancelLogger()
			os.Exit(-1)
		}
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
		svc := services.NetworkPluginProvider{}
		if len(os.Args) >= 2 {
			if os.Args[1] == "--test" {
				err = svc.Launch(name, true, binderClient, logger)
			} else {
				err = svc.Launch(name, false, binderClient, logger)
			}
		} else {
			err = svc.Launch(name, false, binderClient, logger)
		}
		if err != nil {
			logger.Crit("Plugin encountered a fatal error", "type", name, "error", err)
			fmt.Fprintln(os.Stderr, "Plugin encountered a fatal error:", err)
			time.Sleep(100 * time.Millisecond) // give a chance for the log message to be transmitted
			cancelLogger()
			os.Exit(-1)
		} else {
			cancelLogger()
		}

	default:
		if sys.CapabilitiesSupported {
			runtime.LockOSThread()
			// very early we drop most of linux capabilities
			applied, err := sys.Predrop()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(-1)
				return
			}
			if applied {
				exe, err := os.Executable()
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(-1)
				}
				err = syscall.Exec(exe, os.Args, os.Environ())
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error executing self", err)
					os.Exit(-1)
				}
			}

		}
		cmd.Execute()
	}
}
