package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/cmd"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

func main() {

	getLogger := func(name string, handle int) log15.Logger {
		loggerConn, err := net.FileConn(os.NewFile(uintptr(handle), "logger"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		err = loggerConn.(*net.UnixConn).SetReadBuffer(65536)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		err = loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		return utils.NewRemoteLogger(context.Background(), loggerConn).New("proc", name)
	}

	switch name := os.Args[0]; name {
	case "skewer-tcp", "skewer-udp", "skewer-relp", "skewer-journal":
		var binderClient *sys.BinderClient
		logger := log15.New()
		if os.Getenv("HAS_BINDER") == "TRUE" {
			if os.Getenv("HAS_LOGGER") == "TRUE" {
				logger = getLogger(name, 4)
			}
			binderClient, _ = sys.NewBinderClient(os.NewFile(3, "binder"), logger)
		} else if os.Getenv("HAS_LOGGER") == "TRUE" {
			logger = getLogger(name, 3)
		}
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
		svc := services.NetworkPluginProvider{}
		if len(os.Args) >= 2 {
			if os.Args[1] == "--test" {
				svc.Launch(name, true, binderClient, logger)
				return
			}
		}
		svc.Launch(name, false, binderClient, logger)

	case "skewer-audit":
	default:
		if sys.CapabilitiesSupported {
			err := sys.Predrop()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
		}
		cmd.Execute()
	}
}
