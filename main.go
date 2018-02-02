package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"runtime/debug"
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
	"github.com/stephane-martin/skewer/services/base"
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

type fatalErr struct {
	message  string
	err      error
	exitCode int
}

func (e *fatalErr) Error() string {
	if e == nil {
		return ""
	}
	if e.err == nil {
		return e.message
	}
	if len(e.message) == 0 {
		return e.err.Error()
	}
	return fmt.Sprintf("%s: %s", e.message, e.err.Error())
}

func newFatalErr(msg string, err error, exitCode int) *fatalErr {
	if len(msg) == 0 && err == nil {
		return nil
	}
	e := fatalErr{
		message:  msg,
		err:      err,
		exitCode: exitCode,
	}
	if exitCode == 0 {
		e.exitCode = -1
	}
	return &e
}

func dopanic(msg string, err error, exitCode int) {
	e := newFatalErr(msg, err, exitCode)
	if e == nil {
		return
	}
	panic(e)
}

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
	_ = conn.SetReadBuffer(65536)
	_ = conn.SetWriteBuffer(65536)
	return conn
}

func getLogger(ctx context.Context, name string, secret *memguard.LockedBuffer, handle uintptr) (log15.Logger, error) {
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

type reExecType int

const (
	earlyCapsDrop reExecType = iota
	netBindCapDrop
	parentCapsDrop
)

func reExec(t reExecType) {
	switch t {
	case earlyCapsDrop:
		if capabilities.CapabilitiesSupported {
			runtime.LockOSThread()
			// very early we drop most of linux capabilities

			applied, err := capabilities.Predrop()
			if err != nil {
				dopanic("Error pre-dropping capabilities", err, -1)
			}

			if applied {
				exe, err := osext.Executable()
				if err != nil {
					dopanic("Error getting executable path", err, -1)
				}
				err = syscall.Exec(exe, os.Args, os.Environ())
				if err != nil {
					dopanic("Error re-executing self", err, -1)
				}
			}
		}
	case netBindCapDrop:
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
				dopanic("Error getting executable name", err, -1)
			}
			err = syscall.Exec(exe, append([]string{"skewer-linux-child"}, os.Args[1:]...), os.Environ())
			if err != nil {
				dopanic("Error reexecuting self", err, -1)
			}
		}
	case parentCapsDrop:
		if capabilities.CapabilitiesSupported {
			// under Linux, re-exec ourself immediately with fewer privileges
			runtime.LockOSThread()
			_, err := parseCobraFlags()
			if err != nil && err != pflag.ErrHelp {
				dopanic("Error parsing flags", err, -1)
			}
			needFix, err := capabilities.NeedFixLinuxPrivileges(cmd.UidFlag, cmd.GidFlag)
			if err != nil {
				dopanic("Error dropping privileges", err, -1)
			}
			if needFix {
				err = capabilities.FixLinuxPrivileges(cmd.UidFlag, cmd.GidFlag)
				if err != nil {
					dopanic("Error dropping privileges", err, -1)
				}
				err = capabilities.NoNewPriv()
				if err != nil {
					dopanic("NoNewPriv error", err, -1)
				}
				exe, err := os.Executable()
				if err != nil {
					dopanic("Error finding executable name", err, -1)
				}
				err = syscall.Exec(exe, append([]string{"skewer-parent-dropped"}, os.Args[1:]...), os.Environ())
				if err != nil {
					dopanic("Error reexecuting self", err, -1)
				}
			}
		}
	default:
	}
}

func execChild() error {
	err := scomp.SetupSeccomp(-1)
	if err != nil {
		dopanic("Error setting up seccomp", err, -1)
	}

	err = scomp.SetupPledge(-1)
	if err != nil {
		dopanic("Error setting up pledge", err, -1)
	}

	_, err = parseCobraFlags()
	if err != nil && err != pflag.ErrHelp {
		dopanic("Error parsing flags", err, -1)
	}
	return cmd.ExecuteChild()
}

func execServeParent() (status int) {

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
		dopanic("Can't make a new keyring", err, -1)
	}

	boxsecret, err := ring.NewBoxSecret()
	if err != nil {
		dopanic("Can't create a new box secret", err, -1)
	}

	defer func() {
		//boxsecret.Destroy()
		_ = ring.DeleteBoxSecret()
		_ = ring.Destroy()
	}()

	if !cmd.DumpableFlag {
		err := dumpable.SetNonDumpable()
		if err != nil {
			dopanic("Error setting PR_SET_DUMPABLE", err, -1)
		}
	}

	numuid, numgid, err := sys.LookupUid(cmd.UidFlag, cmd.GidFlag)
	if err != nil {
		dopanic("Error looking up uid", err, -1)
	}
	if numuid == 0 {
		dopanic("Provide a non-privileged user with --uid flag", nil, -1)
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
			dopanic("Can't create the required socketpairs", err, -1)
		}
	}

	binderParents := []uintptr{}
	for _, s := range binderSockets {
		binderParents = append(binderParents, s.parent)
	}
	binderCtx, binderCancel := context.WithCancel(context.Background())
	binderWg, err := binder.Server(binderCtx, binderParents, boxsecret, logger) // returns immediately
	if err != nil {
		dopanic("Error setting the root binder", err, -1)
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
		dopanic("Error getting executable name", err, -1)
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
		dopanic("Error creating ring-secret pipe", err, -1)
	}
	extraFiles = append(extraFiles, rRingSecretPipe)
	// the dead man pipe is used by the child to detect that the parent had disappeared
	rDeadManPipe, wDeadManPipe, err := os.Pipe()
	if err != nil {
		dopanic("Error creating dead man pipe", err, -1)
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
		dopanic("Error starting child", err, -1)
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

	state, _ := childProcess.Process.Wait()
	_ = wDeadManPipe.Close()
	if code, ok := state.Sys().(syscall.WaitStatus); ok {
		return code.ExitStatus()
	}
	return 0
}

func execParent() error {
	err := scomp.SetupSeccomp(-1)
	if err != nil {
		dopanic("Error setting up seccomp", err, -1)
	}

	err = scomp.SetupPledge(-1)
	if err != nil {
		dopanic("Error setting up pledge", err, -1)
	}

	name, err := parseCobraFlags()
	if err != nil && err != pflag.ErrHelp {
		dopanic("Error parsing flags", err, -1)
	}
	if name == "serve" && err != pflag.ErrHelp {
		status := execServeParent()
		if status != 0 {
			return fmt.Errorf("Child exited with non-zero status code: '%d'", status)
		}
		return nil
	} else {
		err = cmd.Execute()
	}
	return err
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

func main() {
	defer func() {
		memguard.DestroyAll()
		e := recover()
		if e == nil {
			os.Stdout.Close()
			os.Stderr.Close()
			os.Exit(0)
			return
		}
		if err, ok := e.(*fatalErr); ok {
			fmt.Fprintln(os.Stderr, err.Error())
			fmt.Fprintln(os.Stderr, string(debug.Stack()))
			os.Stdout.Close()
			os.Stderr.Close()
			os.Exit(err.exitCode)
			return
		}
		panic(e)
	}()
	doMain()
}

var cancelLogger context.CancelFunc
var loggerCtx context.Context

func init() {
	loggerCtx, cancelLogger = context.WithCancel(context.Background())
}

func runUnconfined(t base.Types) {
	name, err := base.Name(t, false)
	if err != nil {
		dopanic("Unknown process name", nil, -1)
	}
	sid := os.Getenv("SKEWER_SESSION")

	switch t {
	case base.Configuration:
		dumpable.SetNonDumpable()
		capabilities.NoNewPriv()
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
		var loggerHandle uintptr = 3
		var ringSecret *memguard.LockedBuffer
		rPipe := os.NewFile(4, "ringsecretpipe")
		buf := make([]byte, 32)
		_, err := rPipe.Read(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not read ring secret:", err)
			os.Exit(-1)
		}
		ringSecret, err = memguard.NewImmutableFromBytes(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not create ring secret:", err)
			os.Exit(-1)
		}
		_ = rPipe.Close()
		sessionID := utils.MyULID(ulid.MustParse(sid))
		ring := kring.GetRing(kring.RingCreds{Secret: ringSecret, SessionID: sessionID})

		boxsecret, err := ring.GetBoxSecret()
		if err != nil {
			fmt.Fprintln(os.Stderr, "kring getboxsecret error", err)
			os.Exit(-1)
		}

		logger, err := getLogger(loggerCtx, name, boxsecret, loggerHandle)
		if err != nil {
			dopanic("Could not create a logger for the configuration service", err, -1)
		}
		err = scomp.SetupSeccomp(t)
		if err != nil {
			dopanic("Seccomp setup error", err, -1)
		}
		err = scomp.SetupPledge(t)
		if err != nil {
			dopanic("Pledge setup error", err, -1)
		}
		err = services.LaunchConfProvider(ring, os.Getenv("SKEWER_CONFINED") == "TRUE", logger)
		if err != nil {
			dopanic("ConfigurationProvider encountered a fatal error", err, -1)
		}
		return

	case base.TCP,
		base.UDP,
		base.Graylog,
		base.RELP,
		base.DirectRELP,
		base.Journal,
		base.Store,
		base.Accounting,
		base.KafkaSource,
		base.Filesystem:

		if t == base.Store {
			runtime.GOMAXPROCS(128)
		}
		signal.Ignore(syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
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
			fmt.Fprintln(os.Stderr, "Could not read ring secret:", err)
			os.Exit(-1)
		}
		ringSecret, err = memguard.NewImmutableFromBytes(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not create ring secret:", err)
			os.Exit(-1)
		}
		_ = rPipe.Close()
		sessionID := utils.MyULID(ulid.MustParse(sid))
		ring := kring.GetRing(kring.RingCreds{Secret: ringSecret, SessionID: sessionID})

		boxsecret, err := ring.GetBoxSecret()
		if err != nil {
			fmt.Fprintln(os.Stderr, "kring getboxsecret error", err)
			os.Exit(-1)
		}

		if loggerHdl > 0 {
			logger, err = getLogger(loggerCtx, name, boxsecret, loggerHdl)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Could not create logger for plugin:", err)
				os.Exit(-1)
			}
		}
		if binderHdl > 0 {
			binderClient, _ = binder.NewClient(os.NewFile(binderHdl, "bfile"), boxsecret, logger)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Could not create binder for plugin:", err)
				os.Exit(-1)
			}
		}
		if pipeHdl > 0 {
			pipe = os.NewFile(pipeHdl, "pipe")
		}

		err = scomp.SetupSeccomp(t)
		if err != nil {
			dopanic("Seccomp setup error", err, -1)
		}
		err = scomp.SetupPledge(t)
		if err != nil {
			dopanic("Pledge setup error", err, -1)
		}
		err = services.Launch(
			t,
			services.SetConfined(os.Getenv("SKEWER_CONFINED") == "TRUE"),
			services.SetProfile(os.Getenv("SKEWER_PROFILE") == "TRUE"),
			services.SetRing(ring),
			services.SetBinder(binderClient),
			services.SetLogger(logger),
			services.SetPipe(pipe),
		)
		if err != nil {
			dopanic("Plugin encountered a fatal error", err, -1)
		}
		return
	default:
		dopanic("Unknown process name", nil, -1)
	}

}

func runConfined(t base.Types) {
	name, err := base.Name(t, true)
	if err != nil {
		dopanic("Unknown process name", nil, -1)
	}
	switch t {
	case base.Journal:
		// journal is a special case, as /run/log/journal and /var/log/journal
		// can not be bind-mounted (probably because of setgid bits)
		dumpable.SetNonDumpable()
		path, err := osext.Executable()
		if err != nil {
			dopanic("Error getting executable path", err, -1)
		}
		// mask most of directories, but no pivotroot
		err = namespaces.SetJournalFs(path)
		if err != nil {
			dopanic("mount error", err, -1)
		}
		err = sys.SetHostname(name)
		if err != nil {
			dopanic("sethostname error", err, -1)
		}
		// from here we don't need root capabilities in the container
		err = capabilities.DropAllCapabilities()
		if err != nil {
			dopanic("Error dropping caps", err, -1)
		}
		err = syscall.Exec(path, []string{os.Args[0][9:]}, os.Environ())
		if err != nil {
			dopanic("execve error", err, -1)
		}

	case base.Accounting,
		base.TCP,
		base.UDP,
		base.Graylog,
		base.RELP,
		base.DirectRELP,
		base.Store,
		base.Configuration,
		base.KafkaSource,
		base.Filesystem:

		path, err := osext.Executable()
		if err != nil {
			dopanic("Error getting executable path", err, -1)
		}
		// we are root inside the user namespace that was created by plugin control
		dumpable.SetNonDumpable()

		root, err := namespaces.MakeChroot(path)
		if err != nil {
			dopanic("mount error", err, -1)
		}
		err = namespaces.PivotRoot(root)
		if err != nil {
			dopanic("pivotroot error", err, -1)
		}
		err = sys.SetHostname(name)
		if err != nil {
			dopanic("sethostname error", err, -1)
		}
		// from here we don't need root capabilities in the container
		err = capabilities.DropAllCapabilities()
		if err != nil {
			dopanic("Error dropping caps", err, -1)
		}
		environ := append(os.Environ(), "SKEWER_CONFINED=TRUE")
		err = syscall.Exec(path, []string{os.Args[0][9:]}, environ)
		if err != nil {
			dopanic("execve error", err, -1)
		}
	default:
		dopanic("Unknown process name", nil, -1)
	}
}

func doMain() {
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
		dopanic("Unknown process name", nil, -1)
	}

	if runtime.GOOS == "openbsd" {
		// so that we execute IP capabilities probes before the call to pledge
		_, _ = net.Dial("udp4", "127.0.0.1:80")
	}

	switch name {
	case "skewer-child":
		if runtime.GOOS == "linux" {
			reExec(netBindCapDrop)
			return
		}
		err := execChild()
		if err != nil {
			dopanic("Error executing child", err, -1)
		}
		return

	case "skewer-linux-child":
		err := execChild()
		if err != nil {
			dopanic("Error executing linux child", err, -1)
		}
		return

	case "skewer-parent-dropped":
		err := execParent()
		if err != nil {
			dopanic("Error executing linux parent", err, -1)
		}
		return

	case "skewer":
		if runtime.GOOS == "linux" {
			reExec(earlyCapsDrop)  // we drop most of caps on linux before we even parse the command line
			reExec(parentCapsDrop) // now we can setuid, setgid
		}
		err := execParent()
		if err != nil {
			dopanic("Error executing parent", err, -1)
		}
		return

	default:
	}

	typ, cfnd, err := base.Type(name)
	if err != nil {
		dopanic(fmt.Sprintf("Unknown process name: '%s'", name), nil, -1)
	}

	if cfnd {
		runConfined(typ)
		return
	}
	runUnconfined(typ)
	return
}
