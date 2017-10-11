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
	"github.com/kardianos/osext"
	"github.com/stephane-martin/skewer/cmd"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/dumpable"
	"github.com/stephane-martin/skewer/sys/namespaces"
	"github.com/stephane-martin/skewer/sys/scomp"
	"github.com/stephane-martin/skewer/utils/logging"
)

func getLogger(ctx context.Context, name string, handle int) (log15.Logger, error) {
	loggerConn, err := net.FileConn(os.NewFile(uintptr(handle), "logger"))
	if err != nil {
		return nil, err
	}
	err = loggerConn.(*net.UnixConn).SetReadBuffer(65536)
	if err != nil {
		return nil, err
	}
	err = loggerConn.(*net.UnixConn).SetWriteBuffer(65536)
	if err != nil {
		return nil, err
	}
	return logging.NewRemoteLogger(ctx, loggerConn).New("proc", name), nil
}

func main() {
	name := os.Args[0]
	loggerCtx, cancelLogger := context.WithCancel(context.Background())
	var logger log15.Logger = nil

	if runtime.GOOS == "openbsd" {
		// so that we execute IP capabilities probes before the call to pledge
		net.Dial("udp4", "127.0.0.1:80")
	}

	cleanup := func(msg string, err error) {
		if err != nil {
			if len(msg) == 0 {
				msg = "Process fatal error"
			}
			if logger != nil {
				logger.Crit(msg, "error", err)
			}
			fmt.Fprintln(os.Stderr, name, ":", msg, ":", err.Error())
			time.Sleep(100 * time.Millisecond)
			cancelLogger()
			time.Sleep(100 * time.Millisecond)
			os.Exit(-1)
		} else {
			cancelLogger()
			time.Sleep(100 * time.Millisecond)
		}
	}

	switch name {
	case "skewer-conf":
		sys.MlockAll()
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

		logger, err := getLogger(loggerCtx, name, 3)
		if err != nil {
			cleanup("Could not create a logger for the configuration service", err)
		}
		err = scomp.SetupSeccomp(name)
		if err != nil {
			cleanup("Seccomp setup error", err)
		}
		err = scomp.SetupPledge(name)
		if err != nil {
			cleanup("Pledge setup error", err)
		}
		err = services.LaunchConfProvider(logger)
		if err != nil {
			cleanup("ConfigurationProvider encountered a fatal error", err)
		}

	case "confined-skewer-journal":
		// journal is a special case, as /run/log/journal and /var/log/journal
		// can not be bind-mounted (probably because of setgid bits)
		dumpable.SetNonDumpable()
		path, err := osext.Executable()
		if err != nil {
			cleanup("Error getting executable path", err)
		}
		// mask most of directories, but no pivotroot
		err = namespaces.SetJournalFs(path)
		if err != nil {
			cleanup("mount error", err)
		}
		err = sys.SetHostname(name)
		if err != nil {
			cleanup("sethostname error", err)
		}
		// from here we don't need root capabilities in the container
		err = capabilities.DropAllCapabilities()
		if err != nil {
			cleanup("Error dropping caps", err)
		}
		err = syscall.Exec(path, []string{os.Args[0][9:]}, os.Environ())
		if err != nil {
			cleanup("execve error", err)
		}

	case "confined-skewer-tcp", "confined-skewer-udp", "confined-skewer-relp", "confined-skewer-store", "confined-skewer-conf":
		path, err := osext.Executable()
		if err != nil {
			cleanup("Error getting executable path", err)
		}
		if capabilities.CapabilitiesSupported {
			// we are root inside the user namespace that was created by plugin control
			dumpable.SetNonDumpable()

			root, err := namespaces.MakeChroot(path)
			if err != nil {
				cleanup("mount error", err)
			}
			err = namespaces.PivotRoot(root)
			if err != nil {
				cleanup("pivotroot error", err)
			}
			err = sys.SetHostname(name)
			if err != nil {
				cleanup("sethostname error", err)
			}
			// from here we don't need root capabilities in the container
			err = capabilities.DropAllCapabilities()
			if err != nil {
				cleanup("Error dropping caps", err)
			}
		}

		err = syscall.Exec(path, []string{os.Args[0][9:]}, os.Environ())
		if err != nil {
			cleanup("execve error", err)
		}

	case "skewer-tcp", "skewer-udp", "skewer-relp", "skewer-journal", "skewer-store", "skewer-accounting":
		if name != "skewer-store" {
			sys.MlockAll()
		}
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()

		var binderClient *binder.BinderClient
		var pipe *os.File
		var err error

		if os.Getenv("SKEWER_HAS_BINDER") == "TRUE" {
			if os.Getenv("SKEWER_HAS_LOGGER") == "TRUE" {
				logger, err = getLogger(loggerCtx, name, 4)
				if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
					pipe = os.NewFile(5, "pipe")
				}
			} else if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
				pipe = os.NewFile(4, "pipe")
			}
			binderClient, _ = binder.NewBinderClient(os.NewFile(3, "binder"), logger)
		} else if os.Getenv("SKEWER_HAS_LOGGER") == "TRUE" {
			logger, err = getLogger(loggerCtx, name, 3)
			if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
				pipe = os.NewFile(4, "pipe")
			}
		} else if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
			pipe = os.NewFile(3, "pipe")
		}

		if err != nil {
			cleanup("Could not create logger for plugin", err)
		}

		err = scomp.SetupSeccomp(name)
		if err != nil {
			cleanup("Seccomp setup error", err)
		}
		err = scomp.SetupPledge(name)
		if err != nil {
			cleanup("Pledge setup error", err)
		}
		err = services.Launch(services.NetworkServiceMap[name], os.Getenv("SKEWER_TEST") == "TRUE", binderClient, logger, pipe)
		if err != nil {
			cleanup("Plugin encountered a fatal error", err)
		}

	default:
		sys.MlockAll()
		if capabilities.CapabilitiesSupported {
			runtime.LockOSThread()
			// very early we drop most of linux capabilities

			applied, err := capabilities.Predrop()
			if err != nil {
				cleanup("Error pre-dropping capabilities", err)
			}

			if applied {
				exe, err := osext.Executable()
				if err != nil {
					cleanup("Error getting executable path", err)
				}
				err = syscall.Exec(exe, os.Args, os.Environ())
				if err != nil {
					cleanup("Error re-executing self", err)
				}
			}

		}
		err := scomp.SetupSeccomp("parent")
		if err != nil {
			cleanup("Error setting up seccomp", err)
		}

		err = scomp.SetupPledge("parent")
		if err != nil {
			cleanup("Error setting up pledge", err)
		}

		cmd.Execute()
	}
	cleanup("", nil)
}
