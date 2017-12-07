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
	"github.com/oklog/ulid"
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
	"github.com/stephane-martin/skewer/utils/logging"
)

type spair struct {
	child  uintptr
	parent uintptr
}

func getSocketPair(typ int) (spair, error) {
	a, b, err := sys.SocketPair(typ)
	if err != nil {
		return spair{}, fmt.Errorf("socketpair error: %s", err)
	}
	return spair{child: uintptr(a), parent: uintptr(b)}, nil
}

func getLoggerConn(handle uintptr) (loggerConn *net.UnixConn) {
	c, _ := net.FileConn(os.NewFile(handle, "logger"))
	conn := c.(*net.UnixConn)
	conn.SetReadBuffer(65536)
	conn.SetWriteBuffer(65536)
	return conn
}

func getLogger(ctx context.Context, name string, ring kring.Ring, handle uintptr) (log15.Logger, error) {
	secret, err := ring.GetBoxSecret()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kring getboxsecret error", err)
		return nil, err
	}

	conn, err := net.FileConn(os.NewFile(handle, "logger"))
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

func execServeParent() (err error) {

	/*
		err := capabilities.NoNewPriv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
	*/

	rootlogger := logging.SetLogging(nil, cmd.LoglevelFlag, cmd.LogjsonFlag, cmd.SyslogFlag, cmd.LogfilenameFlag)
	logger := rootlogger.New("proc", "parent")

	ring, err := kring.NewRing()
	if err != nil {
		return makeErr("Can't make a new keyring", err)
	}

	boxsecret, err := ring.NewBoxSecret()
	if err != nil {
		return makeErr("Can't create a new box secret", err)
	}

	defer func() {
		boxsecret.Destroy()
		ring.DeleteBoxSecret()
		ring.Destroy()
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
		if h.Type == cmd.BINDER {
			binderSockets[h.Service], err = getSocketPair(syscall.SOCK_STREAM)
		} else {
			loggerSockets[h.Service], err = getSocketPair(syscall.SOCK_DGRAM)
		}
		if err != nil {
			return makeErr("Can't create the required socketpairs", err)
		}
	}

	binderParents := []uintptr{}
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
	defer func() {
		loggingCancel()
		loggingWg.Wait()
	}()

	logger.Debug("Target user", "uid", numuid, "gid", numgid)

	// execute child under the new user
	exe, err := osext.Executable() // custom Executable function to support OpenBSD
	if err != nil {
		return makeErr("Error getting executable name", err)
	}

	extraFiles := []*os.File{}
	for _, h := range cmd.Handles {
		if h.Type == cmd.BINDER {
			extraFiles = append(extraFiles, os.NewFile(binderSockets[h.Service].child, h.Service))
		} else {
			extraFiles = append(extraFiles, os.NewFile(loggerSockets[h.Service].child, h.Service))
		}
	}
	rOpenBsdSecretPipe, wOpenBsdSecretPipe, err := os.Pipe()
	if err != nil {
		return makeErr("Error creating OpenBSD-secret pipe", err)
	}
	extraFiles = append(extraFiles, rOpenBsdSecretPipe)

	childProcess := exec.Cmd{
		Args:       append([]string{"skewer-child"}, os.Args[1:]...),
		Path:       exe,
		Stdin:      nil,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		ExtraFiles: extraFiles,
		Env:        []string{"PATH=/bin:/usr/bin", fmt.Sprintf("SKEWER_SESSION=%s", ring.GetSessionID().String())},
	}
	if os.Getuid() != numuid {
		// execute the child with the given uid, gid
		childProcess.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(numuid), Gid: uint32(numgid)}}
	}
	err = childProcess.Start()
	if err != nil {
		return makeErr("Error starting child", err)
	}

	rOpenBsdSecretPipe.Close()
	for _, h := range cmd.Handles {
		if h.Type == cmd.BINDER {
			syscall.Close(int(binderSockets[h.Service].child))
		} else {
			syscall.Close(int(loggerSockets[h.Service].child))
		}
	}
	ring.WriteRingPass(wOpenBsdSecretPipe)
	wOpenBsdSecretPipe.Close()

	sigChan := make(chan os.Signal, 10)
	once := sync.Once{}
	go func() {
		for sig := range sigChan {
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
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGUSR1)
	logger.Debug("PIDs", "parent", os.Getpid(), "child", childProcess.Process.Pid)

	childProcess.Process.Wait()
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
		needFix, err := capabilities.NeedFixLinuxPrivileges(cmd.UidFlag, cmd.GidFlag)
		if err != nil {
			cleanup("Error dropping privileges", err, logger, nil)
		}
		if needFix {
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
	// calculate the process name
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

	sid := os.Getenv("SKEWER_SESSION")

	switch name {
	case services.Types2Names[services.Configuration]:
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
		var loggerHandle uintptr = 3
		var bsdSecret *memguard.LockedBuffer
		rPipe := os.NewFile(4, "bsdpipe")
		buf := make([]byte, 32)
		_, err := rPipe.Read(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not read BSD secret:", err)
			os.Exit(-1)
		}
		bsdSecret, err = memguard.NewImmutableFromBytes(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not create BSD secret:", err)
			os.Exit(-1)
		}
		rPipe.Close()
		sessionID := ulid.MustParse(sid)
		ring := kring.GetRing(kring.RingCreds{Secret: bsdSecret, SessionID: sessionID})

		logger, err := getLogger(loggerCtx, name, ring, loggerHandle)
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
		err = services.LaunchConfProvider(ring, logger)
		if err != nil {
			cleanup("ConfigurationProvider encountered a fatal error", err, logger, cancelLogger)
		}
		cleanup("", nil, logger, cancelLogger)

	case services.Types2ConfinedNames[services.Journal]:
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

	case services.Types2ConfinedNames[services.Accounting],
		services.Types2ConfinedNames[services.TCP],
		services.Types2ConfinedNames[services.UDP],
		services.Types2ConfinedNames[services.RELP],
		services.Types2ConfinedNames[services.Store],
		services.Types2ConfinedNames[services.Configuration],
		services.Types2ConfinedNames[services.KafkaSource]:

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

	case services.Types2Names[services.TCP],
		services.Types2Names[services.UDP],
		services.Types2Names[services.RELP],
		services.Types2Names[services.Journal],
		services.Types2Names[services.Store],
		services.Types2Names[services.Accounting],
		services.Types2Names[services.KafkaSource]:

		if name == "skewer-store" {
			runtime.GOMAXPROCS(128)
		}
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()

		var binderClient *binder.BinderClient
		var pipe *os.File
		var err error
		var handle uintptr = 3
		var binderHandle uintptr
		var loggerHandle uintptr
		var messagePipeHandle uintptr
		var bsdSecretPipeHandle uintptr
		var bsdSecret *memguard.LockedBuffer

		if os.Getenv("SKEWER_HAS_BINDER") == "TRUE" {
			binderHandle = handle
			handle++
		}

		if os.Getenv("SKEWER_HAS_LOGGER") == "TRUE" {
			loggerHandle = handle
			handle++
		}
		if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
			messagePipeHandle = handle
			handle++
		}
		bsdSecretPipeHandle = handle
		rPipe := os.NewFile(bsdSecretPipeHandle, "bsdpipe")
		buf := make([]byte, 32)
		_, err = rPipe.Read(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not read BSD secret:", err)
			os.Exit(-1)
		}
		bsdSecret, err = memguard.NewImmutableFromBytes(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not create BSD secret:", err)
			os.Exit(-1)
		}
		rPipe.Close()
		sessionID := ulid.MustParse(sid)
		ring := kring.GetRing(kring.RingCreds{Secret: bsdSecret, SessionID: sessionID})
		if loggerHandle > 0 {
			logger, err = getLogger(loggerCtx, name, ring, loggerHandle)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Could not create logger for plugin:", err)
				os.Exit(-1)
			}
		}
		if binderHandle > 0 {
			binderClient, _ = binder.NewBinderClient(os.NewFile(binderHandle, "binder"), logger)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Could not create binder for plugin:", err)
				os.Exit(-1)
			}
		}
		if messagePipeHandle > 0 {
			pipe = os.NewFile(messagePipeHandle, "pipe")
		}

		err = scomp.SetupSeccomp(name)
		if err != nil {
			cleanup("Seccomp setup error", err, logger, cancelLogger)
		}
		err = scomp.SetupPledge(name)
		if err != nil {
			cleanup("Pledge setup error", err, logger, cancelLogger)
		}
		err = services.Launch(services.Names2Types[name], os.Getenv("SKEWER_TEST") == "TRUE", ring, binderClient, logger, pipe)
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
