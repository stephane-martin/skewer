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
	"github.com/stephane-martin/skewer/services/base"
	"github.com/stephane-martin/skewer/sys"
	"github.com/stephane-martin/skewer/sys/binder"
	"github.com/stephane-martin/skewer/sys/capabilities"
	"github.com/stephane-martin/skewer/sys/dumpable"
	"github.com/stephane-martin/skewer/sys/kring"
	"github.com/stephane-martin/skewer/sys/namespaces"
	"github.com/stephane-martin/skewer/sys/scomp"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/logging"
)

func fatalError(msg string, err error) error {
	msg = strings.TrimSpace(msg)
	if len(msg) == 0 && err == nil {
		return nil
	}
	if len(msg) == 0 {
		return eerrors.WithTypes(err, "Fatal")
	}
	if err == nil {
		return eerrors.WithTypes(eerrors.New(msg), "Fatal")
	}
	return eerrors.WithTypes(eerrors.Wrap(err, msg), "Fatal")
}

type spair struct {
	child  uintptr
	parent uintptr
}

func getSocketPair(typ int) (spair, error) {
	a, b, err := sys.SocketPair(typ)
	if err != nil {
		return spair{}, eerrors.Wrap(err, "socketpair error")
	}
	return spair{child: uintptr(a), parent: uintptr(b)}, nil
}

func getLoggerConn(handle uintptr) (loggerConn *net.UnixConn) {
	c, _ := net.FileConn(os.NewFile(handle, "logger"))
	conn := c.(*net.UnixConn)
	_ = conn.SetReadBuffer(65536)
	_ = conn.SetWriteBuffer(65536)
	return conn
}

func getLogger(ctx context.Context, name string, secret *memguard.LockedBuffer, handle uintptr) (log15.Logger, error) {
	conn, err := net.FileConn(os.NewFile(handle, "logger"))
	if err != nil {
		return nil, eerrors.Wrap(err, "getLogger failed")
	}
	loggerConn := conn.(*net.UnixConn)
	_ = loggerConn.SetReadBuffer(65536)
	_ = loggerConn.SetWriteBuffer(65536)
	return logging.NewRemoteLogger(ctx, loggerConn, secret).New("proc", name), nil
}

type reExecType int

const (
	earlyCapsDrop reExecType = iota
	netBindCapDrop
	parentCapsDrop
)

func reExec(t reExecType) error {
	switch t {
	case earlyCapsDrop:
		if capabilities.CapabilitiesSupported {
			runtime.LockOSThread()
			// very early we drop most of linux capabilities

			applied, err := capabilities.Predrop()
			if err != nil {
				return fatalError("Error pre-dropping capabilities", err)
			}

			if applied {
				exe, err := osext.Executable()
				if err != nil {
					return fatalError("Error getting executable path", err)
				}
				err = syscall.Exec(exe, os.Args, os.Environ())
				if err != nil {
					return fatalError("Error re-executing self", err)
				}
			}
			return nil
		}
	case netBindCapDrop:
		if capabilities.CapabilitiesSupported {
			// another execve is necessary on Linux to ensure that
			// the following capability drop will be effective on
			// all go threads
			runtime.LockOSThread()
			err := capabilities.DropNetBind()
			if err != nil {
				return fatalError("error dropping netbind caps", err)
			}
			exe, err := osext.Executable()
			if err != nil {
				return fatalError("Error getting executable name", err)
			}
			err = syscall.Exec(exe, append([]string{"skewer-linux-child"}, os.Args[1:]...), os.Environ())
			if err != nil {
				return fatalError("Error reexecuting self", err)
			}
		}
		return nil
	case parentCapsDrop:
		if capabilities.CapabilitiesSupported {
			// under Linux, re-exec ourself immediately with fewer privileges
			runtime.LockOSThread()
			_, err := parseCobraFlags()
			if err != nil && err != pflag.ErrHelp {
				return fatalError("Error parsing flags", err)
			}
			needFix, err := capabilities.NeedFixLinuxPrivileges(cmd.UidFlag, cmd.GidFlag)
			if err != nil {
				return fatalError("Error dropping privileges", err)
			}
			if needFix {
				err = capabilities.FixLinuxPrivileges(cmd.UidFlag, cmd.GidFlag)
				if err != nil {
					return fatalError("Error dropping privileges", err)
				}
				err = capabilities.NoNewPriv()
				if err != nil {
					return fatalError("NoNewPriv error", err)
				}
				exe, err := os.Executable()
				if err != nil {
					return fatalError("Error finding executable name", err)
				}
				err = syscall.Exec(exe, append([]string{"skewer-parent-dropped"}, os.Args[1:]...), os.Environ())
				if err != nil {
					return fatalError("Error reexecuting self", err)
				}
			}
		}
		return nil
	}
	return fatalError("Unknown reExec type", nil)
}

func execChild() error {
	err := scomp.SetupSeccomp(-1)
	if err != nil {
		return fatalError("Error setting up seccomp", err)
	}

	err = scomp.SetupPledge(-1)
	if err != nil {
		return fatalError("Error setting up pledge", err)
	}

	_, err = parseCobraFlags()
	if err != nil && err != pflag.ErrHelp {
		return fatalError("Error parsing flags", err)
	}
	return cmd.ExecuteChild()
}

func execServeParent() error {

	/*
		err := capabilities.NoNewPriv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(-1)
		}
	*/

	rootlogger, err := logging.SetupLogging(nil, cmd.LoglevelFlag, cmd.LogjsonFlag, cmd.SyslogFlag, cmd.LogfilenameFlag)
	if err != nil {
		return fatalError("Error when setting up main logger", err)
	}
	logger := rootlogger.New("proc", "parent")

	ring, err := kring.NewRing()
	if err != nil {
		return fatalError("Can't make a new keyring", err)
	}

	boxsecret, err := ring.NewBoxSecret()
	if err != nil {
		return fatalError("Can't create a new box secret", err)
	}

	defer func() {
		_ = ring.DeleteBoxSecret()
		_ = ring.Destroy()
	}()

	if !cmd.DumpableFlag {
		err := dumpable.SetNonDumpable()
		if err != nil {
			return fatalError("Error setting PR_SET_DUMPABLE", err)
		}
	}

	numuid, numgid, err := sys.LookupUid(cmd.UidFlag, cmd.GidFlag)
	if err != nil {
		return fatalError("Error looking up uid", err)
	}
	if numuid == 0 {
		return fatalError("Provide a non-privileged user with --uid flag", nil)
	}

	binderSockets := map[string]spair{}
	loggerSockets := map[string]spair{}

	for _, h := range base.Handles {
		if h.Type == base.Binder {
			binderSockets[h.Service], err = getSocketPair(syscall.SOCK_STREAM)
		} else {
			loggerSockets[h.Service], err = getSocketPair(syscall.SOCK_DGRAM)
		}
		if err != nil {
			return fatalError("Can't create the required socketpairs", err)
		}
	}

	binderParents := []uintptr{}
	for _, s := range binderSockets {
		binderParents = append(binderParents, s.parent)
	}
	binderCtx, binderCancel := context.WithCancel(context.Background())
	binderWg, err := binder.Server(binderCtx, binderParents, boxsecret, logger) // returns immediately
	if err != nil {
		return fatalError("Error setting the root binder", err)
	}
	defer func() {
		binderCancel()
		binderWg.Wait()
	}()

	remoteLoggerConn := []*net.UnixConn{}
	for _, s := range loggerSockets {
		remoteLoggerConn = append(remoteLoggerConn, getLoggerConn(s.parent))
	}
	logger.Debug("Receiving from remote loggers", "nb", len(remoteLoggerConn))
	loggingWg := logging.LogReceiver(loggerCtx, boxsecret, rootlogger, remoteLoggerConn)
	defer func() {
		cancelLogger()
		loggingWg.Wait()
	}()

	logger.Debug("Target user", "uid", numuid, "gid", numgid)

	// execute child under the new user
	exe, err := osext.Executable() // custom Executable() function to support OpenBSD
	if err != nil {
		return fatalError("Error getting executable name", err)
	}

	extraFiles := []*os.File{}
	for _, h := range base.Handles {
		if h.Type == base.Binder {
			extraFiles = append(extraFiles, os.NewFile(binderSockets[h.Service].child, h.Service))
		} else {
			extraFiles = append(extraFiles, os.NewFile(loggerSockets[h.Service].child, h.Service))
		}
	}
	rRingSecretPipe, wRingSecretPipe, err := os.Pipe()
	if err != nil {
		return fatalError("Error creating ring-secret pipe", err)
	}
	extraFiles = append(extraFiles, rRingSecretPipe)
	// the dead man pipe is used by the child to detect that the parent had disappeared
	rDeadManPipe, wDeadManPipe, err := os.Pipe()
	if err != nil {
		return fatalError("Error creating dead man pipe", err)
	}
	extraFiles = append(extraFiles, rDeadManPipe)

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
		childProcess.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(numuid),
				Gid: uint32(numgid),
			},
		}
	}
	err = childProcess.Start()
	_ = rRingSecretPipe.Close()
	_ = rDeadManPipe.Close()
	if err != nil {
		return fatalError("Error starting child", err)
	}

	for _, h := range base.Handles {
		if h.Type == base.Binder {
			_ = syscall.Close(int(binderSockets[h.Service].child))
		} else {
			_ = syscall.Close(int(loggerSockets[h.Service].child))
		}
	}
	_ = ring.WriteRingPass(wRingSecretPipe)
	_ = wRingSecretPipe.Close()

	sigChan := make(chan os.Signal, 10)
	once := sync.Once{}
	go func() {
		for sig := range sigChan {
			logger.Debug("parent received signal", "signal", sig)
			switch sig {
			case syscall.SIGTERM:
				once.Do(func() { _ = childProcess.Process.Signal(sig) })
				return
			case syscall.SIGHUP:
				// reload configuration
				_ = childProcess.Process.Signal(sig)
			case syscall.SIGUSR1:
				// log rotation
				logging.SetupLogging(rootlogger, cmd.LoglevelFlag, cmd.LogjsonFlag, cmd.SyslogFlag, cmd.LogfilenameFlag)
				logging.SetupLogging(logger, cmd.LoglevelFlag, cmd.LogjsonFlag, cmd.SyslogFlag, cmd.LogfilenameFlag)
				logger.Info("log rotation")
			case syscall.SIGINT:
			default:
				logger.Info("Unsupported signal", "signal", sig)
			}
		}
	}()
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGUSR1)
	logger.Debug("PIDs", "parent", os.Getpid(), "child", childProcess.Process.Pid)

	state, _ := childProcess.Process.Wait()
	_ = wDeadManPipe.Close()
	if code, ok := state.Sys().(syscall.WaitStatus); ok {
		status := code.ExitStatus()
		if status == 0 {
			return nil
		}
		return fatalError("Serve child returned a non-zero exit status", nil)
	}
	return nil
}

func execParent() error {
	err := scomp.SetupSeccomp(-1)
	if err != nil {
		return fatalError("Error setting up seccomp", err)
	}

	err = scomp.SetupPledge(-1)
	if err != nil {
		return fatalError("Error setting up pledge", err)
	}

	name, err := parseCobraFlags()
	if err != nil && err != pflag.ErrHelp {
		return fatalError("Error parsing flags", err)
	}
	if name == "serve" && err != pflag.ErrHelp {
		return execServeParent()
	}
	return cmd.Execute()
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

func exit(code int) {
	memguard.DestroyAll()
	os.Stdout.Close()
	os.Stderr.Close()
	os.Exit(code)
}

func main() {
	defer func() {
		e := recover()
		if e == nil {
			exit(0)
		}
		if err, ok := e.(error); ok {
			if eerrors.Is("Fatal", err) {
				fmt.Fprintln(os.Stderr, err.Error())
				exit(1)
			}
		}
		panic(e)
	}()

	err := doMain()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		exit(1)
	}
	exit(0)
}

var cancelLogger context.CancelFunc
var loggerCtx context.Context

func init() {
	loggerCtx, cancelLogger = context.WithCancel(context.Background())
}

func runUnconfined(t base.Types) error {
	name, err := base.Name(t, false)
	if err != nil {
		return fatalError("Unknown process name", nil)
	}
	sid := os.Getenv("SKEWER_SESSION")
	ctx, cancel := context.WithCancel(context.Background())

	signal.Ignore(syscall.SIGHUP, syscall.SIGINT)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGTERM)
	go func() {
		sig := <-sigchan
		if sig != nil {
			cancel()
		}
	}()

	switch t {
	case base.Configuration:
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()

		var loggerHandle uintptr = 3
		var ringSecret *memguard.LockedBuffer
		rPipe := os.NewFile(4, "ringsecretpipe")
		buf := make([]byte, 32)
		_, err := rPipe.Read(buf)
		if err != nil {
			return fatalError("Could not read ring secret:", err)
		}
		ringSecret, err = memguard.NewImmutableFromBytes(buf)
		if err != nil {
			return fatalError("Could not create ring secret:", err)
		}
		_ = rPipe.Close()
		sessionID := utils.MustParseULID(sid)
		ring := kring.GetRing(kring.RingCreds{Secret: ringSecret, SessionID: sessionID})

		boxsecret, err := ring.GetBoxSecret()
		if err != nil {
			return fatalError("kring getboxsecret error", err)
		}

		logger, err := getLogger(loggerCtx, name, boxsecret, loggerHandle)
		if err != nil {
			return fatalError("Could not create a logger for the configuration service", err)
		}
		err = scomp.SetupSeccomp(t)
		if err != nil {
			return fatalError("Seccomp setup error", err)
		}
		err = scomp.SetupPledge(t)
		if err != nil {
			return fatalError("Pledge setup error", err)
		}
		err = services.LaunchConfProvider(ctx, ring, os.Getenv("SKEWER_CONFINED") == "TRUE", logger)
		if err != nil {
			return fatalError("ConfigurationProvider encountered a fatal error", err)
		}
		return nil

	case base.TCP,
		base.UDP,
		base.Graylog,
		base.RELP,
		base.DirectRELP,
		base.Journal,
		base.Store,
		base.Accounting,
		base.MacOS,
		base.KafkaSource,
		base.Filesystem,
		base.HTTPServer:

		if t == base.Store {
			runtime.GOMAXPROCS(128)
		}
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()

		var binderClient binder.Client
		logger := log15.New()
		var pipe *os.File
		var err error
		var handle uintptr = 3
		var binderHdl uintptr
		var loggerHdl uintptr
		var pipeHdl uintptr
		var ringSecretHdl uintptr
		var ringSecret *memguard.LockedBuffer

		if os.Getenv("SKEWER_HAS_BINDER") == "TRUE" {
			binderHdl = handle
			handle++
		}

		if os.Getenv("SKEWER_HAS_LOGGER") == "TRUE" {
			loggerHdl = handle
			handle++
		}

		if os.Getenv("SKEWER_HAS_PIPE") == "TRUE" {
			pipeHdl = handle
			handle++
		}

		ringSecretHdl = handle
		rPipe := os.NewFile(ringSecretHdl, "ringsecretpipe")
		buf := make([]byte, 32)
		_, err = rPipe.Read(buf)
		if err != nil {
			return fatalError("Could not read ring secret:", err)
		}
		ringSecret, err = memguard.NewImmutableFromBytes(buf)
		if err != nil {
			return fatalError("Could not create ring secret:", err)
		}
		_ = rPipe.Close()
		sessionID := utils.MustParseULID(sid)
		ring := kring.GetRing(kring.RingCreds{Secret: ringSecret, SessionID: sessionID})

		boxsecret, err := ring.GetBoxSecret()
		if err != nil {
			return fatalError("kring getboxsecret error", err)
		}

		if loggerHdl > 0 {
			logger, err = getLogger(loggerCtx, name, boxsecret, loggerHdl)
			if err != nil {
				return fatalError("Could not create logger for plugin:", err)
			}
		}
		if binderHdl > 0 {
			binderClient, _ = binder.NewClient(os.NewFile(binderHdl, "bfile"), boxsecret, logger)
			if err != nil {
				return fatalError("Could not create binder for plugin:", err)
			}
		}
		if pipeHdl > 0 {
			pipe = os.NewFile(pipeHdl, "pipe")
		}

		err = scomp.SetupSeccomp(t)
		if err != nil {
			return fatalError("Seccomp setup error", err)
		}
		err = scomp.SetupPledge(t)
		if err != nil {
			return fatalError("Pledge setup error", err)
		}
		err = services.Launch(
			ctx,
			t,
			services.SetConfined(os.Getenv("SKEWER_CONFINED") == "TRUE"),
			services.SetProfile(os.Getenv("SKEWER_PROFILE") == "TRUE"),
			services.SetRing(ring),
			services.SetBinder(binderClient),
			services.SetLogger(logger),
			services.SetPipe(pipe),
		)
		if err != nil {
			return fatalError("Plugin encountered a fatal error", err)
		}
		return nil
	}
	return fatalError("Unknown process name", nil)
}

func runConfined(t base.Types) error {
	name, err := base.Name(t, true)
	if err != nil {
		return fatalError("Unknown process name", nil)
	}
	switch t {
	case base.Journal:
		// journal is a special case, as /run/log/journal and /var/log/journal
		// can not be bind-mounted (probably because of setgid bits)
		dumpable.SetNonDumpable()
		path, err := osext.Executable()
		if err != nil {
			return fatalError("Error getting executable path", err)
		}
		// mask most of directories, but no pivotroot
		err = namespaces.SetJournalFs(path)
		if err != nil {
			return fatalError("mount error", err)
		}
		err = sys.SetHostname(name)
		if err != nil {
			return fatalError("sethostname error", err)
		}
		// from here we don't need root capabilities in the container
		err = capabilities.DropAllCapabilities()
		if err != nil {
			return fatalError("Error dropping caps", err)
		}
		err = syscall.Exec(path, []string{os.Args[0][9:]}, os.Environ())
		if err != nil {
			return fatalError("execve error", err)
		}
		return nil

	case base.TCP,
		base.UDP,
		base.Graylog,
		base.RELP,
		base.DirectRELP,
		base.Store,
		base.Accounting,
		base.MacOS,
		base.Configuration,
		base.KafkaSource,
		base.Filesystem,
		base.HTTPServer:

		path, err := osext.Executable()
		if err != nil {
			return fatalError("Error getting executable path", err)
		}
		// we are root inside the user namespace that was created by plugin control
		dumpable.SetNonDumpable()

		root, err := namespaces.MakeChroot(path)
		if err != nil {
			return fatalError("mount error", err)
		}
		err = namespaces.PivotRoot(root)
		if err != nil {
			return fatalError("pivotroot error", err)
		}
		err = sys.SetHostname(name)
		if err != nil {
			return fatalError("sethostname error", err)
		}
		// from here we don't need root capabilities in the container
		err = capabilities.DropAllCapabilities()
		if err != nil {
			return fatalError("Error dropping caps", err)
		}
		environ := append(os.Environ(), "SKEWER_CONFINED=TRUE")
		err = syscall.Exec(path, []string{os.Args[0][9:]}, environ)
		if err != nil {
			return fatalError("execve error", err)
		}
		return nil
	}
	return fatalError("Unknown process name", nil)
}

func doMain() error {
	defer func() {
		cancelLogger()
		time.Sleep(100 * time.Millisecond)
	}()

	// calculate the process name
	name := strings.Trim(os.Args[0], "./ \n\r")
	spl := strings.Split(name, "/")
	if len(spl) > 0 {
		name = spl[len(spl)-1]
	} else {
		return fatalError("Unknown process name", nil)
	}

	if runtime.GOOS == "openbsd" {
		// so that we execute IP capabilities probes before the call to pledge
		_, _ = net.Dial("udp4", "127.0.0.1:80")
	}

	switch name {
	case "skewer-child":
		if runtime.GOOS == "linux" {
			return reExec(netBindCapDrop)
		}
		return execChild()

	case "skewer-linux-child":
		return execChild()

	case "skewer-parent-dropped":
		return execParent()

	case "skewer":
		if runtime.GOOS == "linux" {
			err := reExec(earlyCapsDrop) // we drop most of caps on linux before we even parse the command line
			if err != nil {
				return err
			}
			err = reExec(parentCapsDrop) // now we can setuid, setgid
			if err != nil {
				return err
			}
		}
		return execParent()
	}

	typ, cfnd, err := base.Type(name)
	if err != nil {
		return fatalError("Unknown process name", nil)
	}

	if cfnd {
		return runConfined(typ)
	}
	return runUnconfined(typ)
}
