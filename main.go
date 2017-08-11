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

	getLogger := func(name string) log15.Logger {
		loggerConn, _ := net.FileConn(os.NewFile(4, "logger"))
		loggerConn.(*net.UnixConn).SetReadBuffer(65536)
		loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
		return utils.NewRemoteLogger(context.Background(), loggerConn).New("proc", name)
	}

	switch name := os.Args[0]; name {
	case "skewer-tcp", "skewer-udp", "skewer-relp", "skewer-journal":
		logger := getLogger(name)
		binderClient, _ := ssys.NewBinderClient(os.NewFile(3, "binder"), logger)
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
