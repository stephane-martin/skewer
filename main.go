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
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/logging"
)

func main() {
	getLogger := func(ctx context.Context, name string, handle int) (log15.Logger, error) {
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

	switch name := os.Args[0]; name {
	case "skewer-conf":
		sys.SetNonDumpable()
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

		loggerCtx, cancelLogger := context.WithCancel(context.Background())
		logger, err := getLogger(loggerCtx, name, 3)
		if err != nil {
			msg := fmt.Sprintf("Could not create a logger for the configuration service: '%s'", err.Error())
			utils.W(os.Stdout, "starterror", []byte(msg))
			time.Sleep(100 * time.Millisecond)
			cancelLogger()
			os.Exit(-1)
		}

		err = services.LaunchConfProvider(logger)
		if err == nil {
			time.Sleep(100 * time.Millisecond)
			cancelLogger()
		} else {
			logger.Crit("ConfigurationProvider encountered a fatal error", "error", err)
			time.Sleep(100 * time.Millisecond)
			cancelLogger()
			os.Exit(-1)
		}

	case "confined-skewer-journal":
		// journal is a special case, as /run/log/journal and /var/log/journal
		// can not be bind-mounted (probably because of setgid bits)
		sys.SetNonDumpable()
		path, err := osext.Executable()
		if err != nil {
			msg := fmt.Sprintf("Error getting executable path (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}
		// mask most of directories, but no pivotroot
		err = sys.SetJournalFs(path)
		if err != nil {
			msg := fmt.Sprintf("mount error (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}
		err = syscall.Sethostname([]byte(name))
		if err != nil {
			msg := fmt.Sprintf("sethostname error (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}
		// from here we don't need root capabilities in the container
		err = sys.DropAllCapabilities()
		if err != nil {
			msg := fmt.Sprintf("Error dropping caps (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}

		err = syscall.Exec(path, []string{os.Args[0][9:]}, os.Environ())
		if err != nil {
			msg := fmt.Sprintf("execve error (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}

	case "confined-skewer-tcp", "confined-skewer-udp", "confined-skewer-relp", "confined-skewer-store", "confined-skewer-conf":
		// we are root inside the user namespace that was created by plugin control
		sys.SetNonDumpable()
		path, err := osext.Executable()
		if err != nil {
			msg := fmt.Sprintf("Error getting executable path (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}
		root, err := sys.MakeChroot(path)
		if err != nil {
			msg := fmt.Sprintf("mount error (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}
		err = sys.PivotRoot(root)
		if err != nil {
			msg := fmt.Sprintf("pivotroot error (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}
		err = syscall.Sethostname([]byte(name))
		if err != nil {
			msg := fmt.Sprintf("sethostname error (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}
		// from here we don't need root capabilities in the container
		err = sys.DropAllCapabilities()
		if err != nil {
			msg := fmt.Sprintf("Error dropping caps (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}

		err = syscall.Exec(path, []string{os.Args[0][9:]}, os.Environ())
		if err != nil {
			msg := fmt.Sprintf("execve error (%s): %s", name, err)
			utils.W(os.Stdout, "starterror", []byte(msg))
			os.Exit(-1)
		}

	case "skewer-tcp", "skewer-udp", "skewer-relp", "skewer-journal", "skewer-audit", "skewer-store":
		sys.SetNonDumpable()
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

		var binderClient *sys.BinderClient
		var pipe *os.File
		var err error

		loggerCtx, cancelLogger := context.WithCancel(context.Background())
		logger := log15.New()

		if os.Getenv("SKEWER_HAS_BINDER") == "TRUE" {
			if os.Getenv("SKEWER_HAS_LOGGER") == "TRUE" {
				logger, err = getLogger(loggerCtx, name, 4)
				if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
					pipe = os.NewFile(5, "pipe")
				}
			} else if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
				pipe = os.NewFile(4, "pipe")
			}
			binderClient, _ = sys.NewBinderClient(os.NewFile(3, "binder"), logger)
		} else if os.Getenv("SKEWER_HAS_LOGGER") == "TRUE" {
			logger, err = getLogger(loggerCtx, name, 3)
			if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
				pipe = os.NewFile(4, "pipe")
			}
		} else if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
			pipe = os.NewFile(3, "pipe")
		}

		if err != nil {
			msg := fmt.Sprintf("Could not create logger for plugin (%s): '%s'", name, err.Error())
			utils.W(os.Stdout, "starterror", []byte(msg))
			time.Sleep(100 * time.Millisecond)
			cancelLogger()
			os.Exit(-1)
		}

		err = services.Launch(services.NetworkServiceMap[name], os.Getenv("SKEWER_TEST") == "TRUE", binderClient, logger, pipe)

		if err != nil {
			logger.Crit("Plugin encountered a fatal error", "type", name, "error", err)
			time.Sleep(100 * time.Millisecond)
			cancelLogger()
			os.Exit(-1)
		} else {
			time.Sleep(100 * time.Millisecond)
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
				envs := os.Environ()
				args := os.Args
				err = syscall.Exec(exe, args, envs)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error executing self")
					fmt.Fprintln(os.Stderr, "- Capabilities")
					fmt.Fprintln(os.Stderr, sys.GetCaps())
					fmt.Fprintf(os.Stderr, "- Exe='%s'\n", exe)
					fmt.Fprintf(os.Stderr, "- Args='%s'\n", args)
					fmt.Fprintf(os.Stderr, "- Env='%s'\n", envs)
					fmt.Fprintf(os.Stderr, "- Error='%s'\n", err)
					os.Exit(-1)
				}
			}

		}
		cmd.Execute()
	}
}
