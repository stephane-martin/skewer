package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/cmd"
	"github.com/stephane-martin/skewer/services"
	ssys "github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/utils"
)

func main() {
	if ssys.CapabilitiesSupported {
		ssys.Predrop()
	}

	getLogger := func(name string, handle int) log15.Logger {
		loggerConn, _ := net.FileConn(os.NewFile(uintptr(handle), "logger"))
		loggerConn.(*net.UnixConn).SetReadBuffer(65536)
		loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
		return utils.NewRemoteLogger(context.Background(), loggerConn).New("proc", name)
	}

	switch name := os.Args[0]; name {
	case "skewer-tcp", "skewer-udp", "skewer-relp", "skewer-journal":
		var binderClient *ssys.BinderClient
		logger := log15.New()
		if os.Getenv("HAS_BINDER") == "TRUE" {
			if os.Getenv("HAS_LOGGER") == "TRUE" {
				logger = getLogger(name, 4)
			}
			binderClient, _ = ssys.NewBinderClient(os.NewFile(3, "binder"), logger)
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
		cmd.Execute()
	}
}
