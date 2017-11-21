package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/awnumar/memguard"
	"github.com/inconshreveable/log15"
	"github.com/kardianos/osext"
	"github.com/spf13/pflag"
	"github.com/stephane-martin/skewer/cmd"
	"github.com/stephane-martin/skewer/services"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/dumpable"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/sys/namespaces"
	"github.com/stephane-martin/skewer/sys/scomp"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/logging"
)

type spair struct {
	child  int
	parent int
}

func getSocketPair(typ int) (spair, error) {
	a, b, err := sys.SocketPair(typ)
	if err != nil {
		return spair{}, fmt.Errorf("socketpair error: %s", err)
	}
	return spair{child: a, parent: b}, nil
}

func getLoggerConn(handle int) (loggerConn *net.UnixConn) {
	c, _ := net.FileConn(os.NewFile(uintptr(handle), "logger"))
	conn := c.(*net.UnixConn)
	conn.SetReadBuffer(65536)
	conn.SetWriteBuffer(65536)
	return conn
}

func getLogger(ctx context.Context, name string, sessionID string, handle int) (log15.Logger, error) {
	secret, err := kring.GetBoxSecret(sessionID)
	if err != nil {
		return nil, err
	}

	conn, err := net.FileConn(os.NewFile(uintptr(handle), "logger"))
	if err != nil {
		return nil, err
	}
	loggerConn := conn.(*net.UnixConn)
	err = loggerConn.SetReadBuffer(65536)
	if err != nil {
		return nil, err
	}
	err = loggerConn.SetWriteBuffer(65536)
	if err != nil {
		return nil, err
	}
	return logging.NewRemoteLogger(ctx, loggerConn, secret).New("proc", name), nil
}

func cleanup(msg string, err error, logger log15.Logger, cancelLogger context.CancelFunc) {
	if err != nil {
		if len(msg) == 0 {
			msg = "Process fatal error"
		}
		logger.Crit(msg, "error", err)
		time.Sleep(100 * time.Millisecond)
		memguard.SafeExit(-1)
	} else if len(msg) > 0 {
		logger.Crit(msg)
		time.Sleep(100 * time.Millisecond)
		memguard.SafeExit(-1)
	}
	if cancelLogger != nil {
		cancelLogger()
		time.Sleep(100 * time.Millisecond)
	}
	memguard.DestroyAll()
}

func earlyDropCaps(logger log15.Logger) {
	if capabilities.CapabilitiesSupported {
		runtime.LockOSThread()
		// very early we drop most of linux capabilities

		applied, err := capabilities.Predrop()
		if err != nil {
			cleanup("Error pre-dropping capabilities", err, logger, nil)
		}

		if applied {
			exe, err := osext.Executable()
			if err != nil {
				cleanup("Error getting executable path", err, logger, nil)
			}
			err = syscall.Exec(exe, os.Args, os.Environ())
			if err != nil {
				cleanup("Error re-executing self", err, logger, nil)
			}
		}

	}
}

func dropNetBindCap(logger log15.Logger) {
	if capabilities.CapabilitiesSupported {
		// another execve is necessary on Linux to ensure that
		// the following capability drop will be effective on
		// all go threads
		runtime.LockOSThread()
		err := capabilities.DropNetBind()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		exe, err := osext.Executable()
		if err != nil {
			cleanup("Error getting executable name", err, logger, nil)
		}
		err = syscall.Exec(exe, append([]string{"skewer-linux-child"}, os.Args[1:]...), os.Environ())
		if err != nil {
			cleanup("Error reexecuting self", err, logger, nil)
		}
	}
}

func execChild(logger log15.Logger) {
	err := scomp.SetupSeccomp("parent")
	if err != nil {
		cleanup("Error setting up seccomp", err, logger, nil)
	}

	err = scomp.SetupPledge("parent")
	if err != nil {
		cleanup("Error setting up pledge", err, logger, nil)
	}

	_, err = parseCobraFlags()
	if err != nil && err != pflag.ErrHelp {
		cleanup("Error parsing flags", err, logger, nil)
	}

	err = cmd.ExecuteChild()
	if err != nil {
		cleanup("Error executing child", err, logger, nil)
	}
	cleanup("", nil, logger, nil)
}

func makeErr(msg string, err error) error {
	if err == nil {
		return fmt.Errorf(msg)
	}
	return fmt.Errorf("%s: %s", msg, err)
}

func execServeParent() error {

	/*
		err := capabilities.NoNewPriv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
	*/

	rootlogger := logging.SetLogging(nil, cmd.LoglevelFlag, cmd.LogjsonFlag, cmd.SyslogFlag, cmd.LogfilenameFlag)
	logger := rootlogger.New("proc", "parent")

	sessionID := utils.NewUid().String()
	err := kring.JoinSessionKeyRing()
	if err != nil {
		return makeErr("Can't join the linux keyring", err)
	}
	boxsecret, err := kring.NewBoxSecret(sessionID)
	if err != nil {
		return makeErr("Can't create a new box secret", err)
	}

	defer func() {
		boxsecret.Destroy()
		kring.DeleteBoxSecret(sessionID)
		kring.DestroySemaphore(sessionID)
	}()

	if !cmd.DumpableFlag {
		err := dumpable.SetNonDumpable()
		if err != nil {
			return makeErr("Error setting PR_SET_DUMPABLE", err)
		}
	}

	numuid, numgid, err := sys.LookupUid(cmd.UidFlag, cmd.GidFlag)
	if err != nil {
		return makeErr("Error looking up uid", err)
	}
	if numuid == 0 {
		return makeErr("Provide a non-privileged user with --uid flag", nil)
	}

	binderSockets := map[string]spair{}
	loggerSockets := map[string]spair{}

	for _, h := range cmd.Handles {
		if strings.HasSuffix(h, "_BINDER") {
			binderSockets[h], err = getSocketPair(syscall.SOCK_STREAM)
		} else {
			loggerSockets[h], err = getSocketPair(syscall.SOCK_DGRAM)
		}
		if err != nil {
			return makeErr("Can't create the required socketpairs", err)
		}
	}

	binderParents := []int{}
	for _, s := range binderSockets {
		binderParents = append(binderParents, s.parent)
	}
	err = binder.Binder(binderParents, logger) // returns immediately
	if err != nil {
		return makeErr("Error setting the root binder", err)
	}

	remoteLoggerConn := []*net.UnixConn{}
	for _, s := range loggerSockets {
		remoteLoggerConn = append(remoteLoggerConn, getLoggerConn(s.parent))
	}
	loggingCtx, loggingCancel := context.WithCancel(context.Background())
	logger.Debug("Receiving from remote loggers", "nb", len(remoteLoggerConn))
	loggingWg := logging.LogReceiver(loggingCtx, boxsecret, rootlogger, remoteLoggerConn)

	logger.Debug("Target user", "uid", numuid, "gid", numgid)

	// execute child under the new user
	exe, err := osext.Executable() // custom Executable function to support OpenBSD
	if err != nil {
		return makeErr("Error getting executable name", err)
	}

	extraFiles := []*os.File{}
	for _, h := range cmd.Handles {
		if strings.HasSuffix(h, "_BINDER") {
			extraFiles = append(extraFiles, os.NewFile(uintptr(binderSockets[h].child), h))
		} else {
			extraFiles = append(extraFiles, os.NewFile(uintptr(loggerSockets[h].child), h))
		}
	}

	childProcess := exec.Cmd{
		Args:       append([]string{"skewer-child"}, os.Args[1:]...),
		Path:       exe,
		Stdin:      nil,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		ExtraFiles: extraFiles,
		Env:        []string{"PATH=/bin:/usr/bin", fmt.Sprintf("SKEWER_SESSION=%s", sessionID)},
	}
	if os.Getuid() != numuid {
		// execute the child with the given uid, gid
		childProcess.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(numuid), Gid: uint32(numgid)}}
	}
	err = childProcess.Start()
	if err != nil {
		return makeErr("Error starting child", err)
	}

	for _, h := range cmd.Handles {
		if strings.HasSuffix(h, "_BINDER") {
			syscall.Close(binderSockets[h].child)
		} else {
			syscall.Close(loggerSockets[h].child)
		}
	}

	sig_chan := make(chan os.Signal, 10)
	once := sync.Once{}
	go func() {
		for sig := range sig_chan {
			logger.Debug("parent received signal", "signal", sig)
			switch sig {
			case syscall.SIGTERM:
				once.Do(func() { childProcess.Process.Signal(sig) })
				return
			case syscall.SIGHUP:
				// reload configuration
				childProcess.Process.Signal(sig)
			case syscall.SIGUSR1:
				// log rotation
				logging.SetLogging(rootlogger, cmd.LoglevelFlag, cmd.LogjsonFlag, cmd.SyslogFlag, cmd.LogfilenameFlag)
				logging.SetLogging(logger, cmd.LoglevelFlag, cmd.LogjsonFlag, cmd.SyslogFlag, cmd.LogfilenameFlag)
				logger.Info("log rotation")
			default:
				logger.Info("Unsupported signal", "signal", sig)
			}
		}
	}()
	signal.Notify(sig_chan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGUSR1)
	logger.Debug("PIDs", "parent", os.Getpid(), "child", childProcess.Process.Pid)

	childProcess.Process.Wait()
	loggingCancel()
	loggingWg.Wait()
	return nil
}

func execParent(logger log15.Logger) {
	err := scomp.SetupSeccomp("parent")
	if err != nil {
		cleanup("Error setting up seccomp", err, logger, nil)
	}

	err = scomp.SetupPledge("parent")
	if err != nil {
		cleanup("Error setting up pledge", err, logger, nil)
	}

	name, err := parseCobraFlags()
	if err != nil && err != pflag.ErrHelp {
		cleanup("Error parsing flags", err, logger, nil)
	}
	logger.Debug("Executing command", "command", name, "args", strings.Join(os.Args, " "))
	if name == "serve" && err != pflag.ErrHelp {
		err = execServeParent()
	} else {
		err = cmd.Execute()
	}
	if err != nil {
		cleanup("Error executing command", err, logger, nil)
	}
	cleanup("", nil, logger, nil)

}

func parseCobraFlags() (string, error) {
	c, _, err := cmd.RootCmd.Find(os.Args[1:])
	if err != nil {
		return c.Name(), err
	}
	var flags []string
	c, flags, err = c.Find(os.Args[1:])
	if err != nil {
		return c.Name(), err
	}
	return c.Name(), c.ParseFlags(flags)
}

func fixLinuxParentPrivileges(logger log15.Logger) {
	if capabilities.CapabilitiesSupported {
		// under Linux, re-exec ourself immediately with fewer privileges
		runtime.LockOSThread()
		_, err := parseCobraFlags()
		if err != nil && err != pflag.ErrHelp {
			cleanup("Error parsing flags", err, logger, nil)
		}
		need_fix, err := capabilities.NeedFixLinuxPrivileges(cmd.UidFlag, cmd.GidFlag)
		if err != nil {
			cleanup("Error dropping privileges", err, logger, nil)
		}
		if need_fix {
			err = capabilities.FixLinuxPrivileges(cmd.UidFlag, cmd.GidFlag)
			if err != nil {
				cleanup("Error dropping privileges", err, logger, nil)
			}
			err = capabilities.NoNewPriv()
			if err != nil {
				cleanup("NoNewPriv error", err, logger, nil)
			}
			exe, err := os.Executable()
			if err != nil {
				cleanup("Error finding executable name", err, logger, nil)
			}
			err = syscall.Exec(exe, append([]string{"skewer-parent-dropped"}, os.Args[1:]...), os.Environ())
			if err != nil {
				cleanup("Error rexecuting self", err, logger, nil)
			}
		}
	}

}

func main() {
	name := strings.Trim(os.Args[0], "./ \n\r")
	spl := strings.Split(name, "/")
	if len(spl) > 0 {
		name = spl[len(spl)-1]
	} else {
		name = "unknown"
	}

	loggerCtx, cancelLogger := context.WithCancel(context.Background())
	logger := log15.New()
	logger.SetHandler(log15.LvlFilterHandler(log15.LvlWarn, log15.StderrHandler))

	if runtime.GOOS == "openbsd" {
		// so that we execute IP capabilities probes before the call to pledge
		net.Dial("udp4", "127.0.0.1:80")
	}

	sessionID := os.Getenv("SKEWER_SESSION")

	switch name {
	case "skewer-conf":
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

		logger, err := getLogger(loggerCtx, name, sessionID, 3)
		if err != nil {
			cleanup("Could not create a logger for the configuration service", err, logger, cancelLogger)
		}
		err = scomp.SetupSeccomp(name)
		if err != nil {
			cleanup("Seccomp setup error", err, logger, cancelLogger)
		}
		err = scomp.SetupPledge(name)
		if err != nil {
			cleanup("Pledge setup error", err, logger, cancelLogger)
		}
		err = services.LaunchConfProvider(sessionID, logger)
		if err != nil {
			cleanup("ConfigurationProvider encountered a fatal error", err, logger, cancelLogger)
		}
		cleanup("", nil, logger, cancelLogger)

	case "confined-skewer-journal":
		// journal is a special case, as /run/log/journal and /var/log/journal
		// can not be bind-mounted (probably because of setgid bits)
		dumpable.SetNonDumpable()
		path, err := osext.Executable()
		if err != nil {
			cleanup("Error getting executable path", err, logger, cancelLogger)
		}
		// mask most of directories, but no pivotroot
		err = namespaces.SetJournalFs(path)
		if err != nil {
			cleanup("mount error", err, logger, cancelLogger)
		}
		err = sys.SetHostname(name)
		if err != nil {
			cleanup("sethostname error", err, logger, cancelLogger)
		}
		// from here we don't need root capabilities in the container
		err = capabilities.DropAllCapabilities()
		if err != nil {
			cleanup("Error dropping caps", err, logger, cancelLogger)
		}
		err = syscall.Exec(path, []string{os.Args[0][9:]}, os.Environ())
		if err != nil {
			cleanup("execve error", err, logger, cancelLogger)
		}

	case "confined-skewer-accounting", "confined-skewer-tcp", "confined-skewer-udp", "confined-skewer-relp", "confined-skewer-store", "confined-skewer-conf":
		path, err := osext.Executable()
		if err != nil {
			cleanup("Error getting executable path", err, logger, cancelLogger)
		}
		if capabilities.CapabilitiesSupported {
			// we are root inside the user namespace that was created by plugin control
			dumpable.SetNonDumpable()

			root, err := namespaces.MakeChroot(path)
			if err != nil {
				cleanup("mount error", err, logger, cancelLogger)
			}
			err = namespaces.PivotRoot(root)
			if err != nil {
				cleanup("pivotroot error", err, logger, cancelLogger)
			}
			err = sys.SetHostname(name)
			if err != nil {
				cleanup("sethostname error", err, logger, cancelLogger)
			}
			// from here we don't need root capabilities in the container
			err = capabilities.DropAllCapabilities()
			if err != nil {
				cleanup("Error dropping caps", err, logger, cancelLogger)
			}
		}

		err = syscall.Exec(path, []string{os.Args[0][9:]}, os.Environ())
		if err != nil {
			cleanup("execve error", err, logger, cancelLogger)
		}

	case "skewer-tcp", "skewer-udp", "skewer-relp", "skewer-journal", "skewer-store", "skewer-accounting":
		if name == "skewer-store" {
			runtime.GOMAXPROCS(128)
		}
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()

		var binderClient *binder.BinderClient
		var pipe *os.File
		var err error

		if os.Getenv("SKEWER_HAS_BINDER") == "TRUE" {
			if os.Getenv("SKEWER_HAS_LOGGER") == "TRUE" {
				logger, err = getLogger(loggerCtx, name, sessionID, 4)
				if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
					pipe = os.NewFile(5, "pipe")
				}
			} else if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
				pipe = os.NewFile(4, "pipe")
			}
			binderClient, _ = binder.NewBinderClient(os.NewFile(3, "binder"), logger)
		} else if os.Getenv("SKEWER_HAS_LOGGER") == "TRUE" {
			logger, err = getLogger(loggerCtx, name, sessionID, 3)
			if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
				pipe = os.NewFile(4, "pipe")
			}
		} else if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
			pipe = os.NewFile(3, "pipe")
		}

		if err != nil {
			cleanup("Could not create logger for plugin", err, logger, cancelLogger)
		}

		err = scomp.SetupSeccomp(name)
		if err != nil {
			cleanup("Seccomp setup error", err, logger, cancelLogger)
		}
		err = scomp.SetupPledge(name)
		if err != nil {
			cleanup("Pledge setup error", err, logger, cancelLogger)
		}
		err = services.Launch(services.NetworkServiceMap[name], os.Getenv("SKEWER_TEST") == "TRUE", sessionID, binderClient, logger, pipe)
		if err != nil {
			cleanup("Plugin encountered a fatal error", err, logger, cancelLogger)
		}
		cleanup("", nil, logger, cancelLogger)

	case "skewer-child":
		dropNetBindCap(logger)
		execChild(logger)

	case "skewer-linux-child":
		execChild(logger)

	case "skewer-parent-dropped":
		execParent(logger)

	case "skewer":
		// we drop most of caps on linux before we even parse the command line
		earlyDropCaps(logger)
		// now we can setuid, setgid
		fixLinuxParentPrivileges(logger)
		execParent(logger)

	default:
		cleanup("Unknown process name", nil, logger, cancelLogger)
	}
	cleanup("", nil, logger, cancelLogger)
}
