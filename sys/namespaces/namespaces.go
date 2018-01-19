// +build linux

package namespaces

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/EricLagergren/go-gnulib/ttyname"
	"github.com/fatih/set"
	dump "github.com/stephane-martin/skewer/sys/dumpable"
	"github.com/stephane-martin/skewer/utils"
)

func (c *NamespacedCmd) Start() (err error) {
	paths := envPaths{
		certFiles: make([]string, 0),
		certPaths: make([]string, 0),
		polldirs:  make([]string, 0),
	}

	for _, d := range c.polldirs {
		if !utils.IsDir(d) {
			return fmt.Errorf("The path to poll '%s' is not a directory", d)
		}
		abs, err := filepath.Abs(d)
		if err != nil {
			return err
		}
		paths.polldirs = append(paths.polldirs, abs)
	}

	for _, f := range c.certFiles {
		if !utils.FileExists(f) {
			return fmt.Errorf("Certificate file '%s' does not exist", f)
		}
		paths.certFiles = append(paths.certFiles, f)
	}

	for _, f := range c.certPaths {
		if !utils.IsDir(f) {
			return fmt.Errorf("Certificate path '%s' does not exist or is not a directory", f)
		}
		paths.certPaths = append(paths.certPaths, f)
	}

	if len(c.acctPath) > 0 {
		acctPath, err := filepath.Abs(c.acctPath)
		if err != nil {
			return err
		}
		paths.acctParentDir = filepath.Dir(acctPath)
		if !utils.IsDir(paths.acctParentDir) {
			return fmt.Errorf("Accounting path '%s' does not exist or is not a directory", paths.acctParentDir)
		}
	}

	if len(c.fileDestTmpl) > 0 {
		fileDestTmpl, err := filepath.Abs(c.fileDestTmpl)
		if err != nil {
			return err
		}
		// ex for fileDestTmpl: "/var/log/skewer/{{.Fields.Date}}/{{.Fields.Appname}}.log"
		n := strings.Index(fileDestTmpl, "{")
		if n == -1 {
			// static filename, not a template
			paths.fileDestParentDir = filepath.Dir(fileDestTmpl)
		} else {
			// take the prefix, eg "/var/log/skewer"
			paths.fileDestParentDir = strings.TrimRight(fileDestTmpl[:n], "/")
		}
		if !utils.IsDir(paths.fileDestParentDir) {
			return fmt.Errorf("Supposed to write logs to directory '%s', but it does not exist, or is not a directory", paths.fileDestParentDir)
		}

	}

	if len(c.storePath) > 0 {
		paths.storePath, err = filepath.Abs(c.storePath)
		if err != nil {
			return err
		}
		if !utils.IsDir(paths.storePath) {
			return fmt.Errorf("Store path '%s' does not exist, or is not a directory", paths.storePath)
		}
	}

	if len(c.confPath) > 0 {
		paths.confPath, err = filepath.Abs(c.confPath)
		if err != nil {
			return err
		}
		if !utils.IsDir(paths.confPath) {
			return fmt.Errorf("Configuration path '%s' does not exist, or is not a directory", paths.confPath)
		}
	}

	ttyName := ""
	if ttyname.IsAtty(1) {
		var err error
		ttyName, err = ttyname.TtyName(1)
		if err != nil {
			ttyName = ""
		}
	}

	c.cmd.AppendEnv(setupEnv(paths, ttyName))

	c.cmd.SetSysProcAttr(&syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS,

		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	})

	// TODO: DUMPABLE should only be set in the child, just to write uid_map
	if !c.dumpable {
		dump.SetDumpable()
	}
	err = c.cmd.Start()
	if !c.dumpable {
		dump.SetNonDumpable()
	}
	return err
}

func PivotRoot(root string) (err error) {
	oldroot := filepath.Join(root, "oldroot")
	err = utils.Chain(
		func() error { return os.Mkdir(oldroot, 0777) },
		func() error { return syscall.PivotRoot(root, oldroot) },
		func() error { return syscall.Chdir("/") },
		func() error { return syscall.Unmount("/oldroot", syscall.MNT_DETACH) },
		func() error { return os.Remove("/oldroot") },
		func() error { return syscall.Chroot("/newroot") },
		func() error { return os.Chdir("/") },
		func() error { return os.Symlink(filepath.Join("/dev", "pts", "ptmx"), filepath.Join("/dev", "ptmx")) },
	)
	if err != nil {
		err = fmt.Errorf("PivotRoot error: %s", err.Error())
	}
	return err
}

func SetJournalFs(targetExec string) error {
	var err error

	roRemounts := []mountPoint{
		{
			baseMountPoint: baseMountPoint{
				Target: "/bin",
			},
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			baseMountPoint: baseMountPoint{
				Target: "/sbin",
			},
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			baseMountPoint: baseMountPoint{
				Target: "/lib",
			},
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			baseMountPoint: baseMountPoint{
				Target: "/lib64",
			},
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			baseMountPoint: baseMountPoint{
				Target: "/usr",
			},
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			baseMountPoint: baseMountPoint{
				Target: "/var",
			},
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC,
		},
		{
			baseMountPoint: baseMountPoint{
				Target: "/home",
			},
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			baseMountPoint: baseMountPoint{
				Target: "/etc",
			},
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC,
		},
	}

	mounts := []mountPoint{
		{
			baseMountPoint: baseMountPoint{
				Source: "proc",
				Target: "/proc",
			},
			Fs:    "proc",
			Flags: syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV,
		},
		{
			baseMountPoint: baseMountPoint{
				Source: "tmpfs",
				Target: "/dev",
			},
			Fs:    "tmpfs",
			Flags: syscall.MS_NOSUID | syscall.MS_NOEXEC,
			Data:  "mode=755",
		},
		{
			baseMountPoint: baseMountPoint{
				Source: "tmpfs",
				Target: "/boot",
			},
			Fs:    "tmpfs",
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC,
			Data:  "mode=755",
		},
	}

	temp, err := ioutil.TempDir("", "skewer-tmpdev")
	if err != nil {
		return fmt.Errorf("TempDir error: %s", err)
	}

	// we bind-mount /dev on 'temp'
	err = syscall.Mount("/dev", temp, "bind", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		return fmt.Errorf("Error bind-mounting /dev: %s", err)
	}

	// mask /proc, /dev, /boot
	for _, m := range mounts {
		if !utils.FileExists(m.Target) {
			err = os.MkdirAll(m.Target, 0755)
			if err != nil {
				return fmt.Errorf("mkdirall %s error: %v", m.Target, err)
			}
		}
		err = syscall.Mount(m.Source, m.Target, m.Fs, uintptr(m.Flags), "")
		if err != nil {
			return fmt.Errorf("failed to mount %s: %s", m.Target, err.Error())
		}
	}

	// make most of directories read-only
	for _, m := range roRemounts {
		if utils.IsDir(m.Target) {
			err := syscall.Mount(m.Target, m.Target, "bind", syscall.MS_BIND|syscall.MS_REC, "")
			if err != nil {
				return fmt.Errorf("failed to bind-mount %s: %s", m.Target, err.Error())
			}
			err = syscall.Mount(m.Target, m.Target, "bind", uintptr(syscall.MS_BIND|syscall.MS_REC|syscall.MS_REMOUNT|syscall.MS_RDONLY|m.Flags), "")
			if err != nil {
				return fmt.Errorf("failed to remount %s: %s", m.Target, err.Error())
			}
		}
	}

	// populate the new /dev
	devices := []string{"null", "zero", "full", "random", "urandom"}
	for _, device := range devices {
		f, err := os.Create(filepath.Join("/dev", device))
		if err == nil {
			f.Close()
			err = syscall.Mount(filepath.Join(temp, device), filepath.Join("/dev", device), "bind", syscall.MS_BIND, "")
			if err != nil {
				return fmt.Errorf("Error bind-mounting device '%s': %s", device, err)
			}
		}
	}

	// /dev/shm is mounted from parent container, so that Posix IPC and shared mem can work
	err = os.Mkdir("/dev/shm", 0755)
	if err == nil {
		err = syscall.Mount(filepath.Join(temp, "shm"), "/dev/shm", "bind", syscall.MS_BIND|syscall.MS_REC, "")
		if err != nil {
			return fmt.Errorf("Error bind-mounting /dev/shm: %s", err)
		}
	} else {
		return fmt.Errorf("Failed to create /dev/shm: %s", err)
	}

	err = syscall.Unmount(temp, syscall.MNT_DETACH)
	if err != nil {
		return fmt.Errorf("Error unmounting %s: %s", temp, err)
	}
	err = os.Remove(temp)
	if err != nil {
		return fmt.Errorf("Error removing %s: %s", temp, err)
	}

	tmpMP := mountPoint{
		baseMountPoint: baseMountPoint{
			Source: "tmpfs",
			Target: "/tmp",
		},
		Fs:    "tmpfs",
		Flags: syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC,
		Data:  "mode=755",
	}
	err = syscall.Mount(tmpMP.Source, tmpMP.Target, tmpMP.Fs, uintptr(tmpMP.Flags), "")
	if err != nil {
		return fmt.Errorf("failed to mount %s: %s", tmpMP.Target, err)
	}
	return nil
}

func MakeChroot(targetExec string) (string, error) {
	root, err := ioutil.TempDir("", "skewer-confined")
	if err != nil {
		return "", err
	}
	os.Mkdir(root, 0755)

	err = syscall.Mount("", root, "tmpfs", syscall.MS_NODEV|syscall.MS_NOSUID|syscall.MS_NOEXEC, "")
	if err != nil {
		return "", fmt.Errorf("Failed to mount temp root: %s", err)
	}

	os.Chdir(root)
	os.Mkdir("newroot", 0755)

	mounts := []mountPoint{
		{
			baseMountPoint: baseMountPoint{
				Source: "proc",
				Target: "/proc",
			},
			Fs:    "proc",
			Flags: syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV,
		},
		{
			baseMountPoint: baseMountPoint{
				Source: "tmpfs",
				Target: "/dev",
			},
			Fs:    "tmpfs",
			Flags: syscall.MS_NOSUID | syscall.MS_NOEXEC,
			Data:  "mode=755",
		},
		{
			baseMountPoint: baseMountPoint{
				Source: "tmpfs",
				Target: "/run",
			},
			Fs:    "tmpfs",
			Flags: syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOSUID,
			Data:  "mode=755",
		},
		{
			baseMountPoint: baseMountPoint{
				Source: "devpts",
				Target: "/dev/pts",
			},
			Fs:    "devpts",
			Flags: syscall.MS_NOSUID | syscall.MS_NOEXEC,
			Data:  "newinstance,ptmxmode=0666,mode=620",
		},
		{
			baseMountPoint: baseMountPoint{
				Source: "tmpfs",
				Target: "/tmp",
			},
			Fs:    "tmpfs",
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
			Data:  "mode=700",
		},
		{
			baseMountPoint: baseMountPoint{
				Source: "tmpfs",
				Target: "/lib",
			},
			Fs:    "tmpfs",
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
			Data:  "mode=755",
		},
		{
			baseMountPoint: baseMountPoint{
				Source: "tmpfs",
				Target: "/lib64",
			},
			Fs:    "tmpfs",
			Flags: syscall.MS_NOSUID | syscall.MS_NODEV,
			Data:  "mode=755",
		},
	}

	mounted := set.New(set.ThreadSafe)
	for _, m := range mounts {
		target := filepath.Join(root, "newroot", m.Target)
		err := os.MkdirAll(target, 0755)
		if err != nil {
			return "", fmt.Errorf("mkdirall %s error: %v", target, err)
		}
		err = syscall.Mount(m.Source, target, m.Fs, uintptr(m.Flags), m.Data)
		if err != nil {
			return "", fmt.Errorf("failed to mount %s to %s: %v", m.Source, target, err)
		}
		mounted.Add(baseMountPoint{
			Source: "",
			Target: m.Target,
		})
	}

	bindMounts := []bindMountPoint{}

	// bind mount skewer configuration directory
	confDir := strings.TrimSpace(os.Getenv("SKEWER_CONF_DIR"))
	if len(confDir) > 0 {
		bindMounts = append(bindMounts, bindMountPoint{
			baseMountPoint: baseMountPoint{
				Source: confDir,
				Target: filepath.Join(root, "newroot", "tmp", "conf", confDir),
			},
			ReadOnly: true,
			IsDir:    true,
			Flags:    syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOSUID,
		})
	}

	acctDir := strings.TrimSpace(os.Getenv("SKEWER_ACCT_DIR"))
	if len(acctDir) > 0 {
		bindMounts = append(bindMounts, bindMountPoint{
			baseMountPoint: baseMountPoint{
				Source: acctDir,
				Target: filepath.Join(root, "newroot", "tmp", "acct", acctDir),
			},
			ReadOnly: true,
			IsDir:    true,
			Flags:    syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOSUID,
		})
	}

	// bind mount shared libraries from /lib and /lib64
	shared_libs := []string{}
	if _, err := os.Stat("/lib"); err == nil {
		shared_libs = append(shared_libs, myWalk("/lib")...)
	}
	if _, err := os.Stat("/lib64"); err == nil {
		shared_libs = append(shared_libs, myWalk("/lib64")...)
	}

	for _, library := range shared_libs {
		bindMounts = append(bindMounts, bindMountPoint{
			baseMountPoint: baseMountPoint{
				Source: library,
				Target: filepath.Join(root, "newroot", library),
			},
			ReadOnly: true,
			IsDir:    false,
			Flags:    syscall.MS_NOSUID | syscall.MS_NODEV,
		})
	}

	// bind mount some devices in /dev
	devices := []string{"null", "zero", "full", "random", "urandom", "tty"}

	for _, device := range devices {
		bindMounts = append(bindMounts, bindMountPoint{
			baseMountPoint: baseMountPoint{
				Source: filepath.Join("/dev", device),
				Target: filepath.Join(root, "newroot", "dev", device),
			},
			ReadOnly: false,
			IsDir:    false,
			Flags:    syscall.MS_NOSUID | syscall.MS_NOEXEC,
		})
	}

	// bind mount /dev/shm
	bindMounts = append(bindMounts, bindMountPoint{
		baseMountPoint: baseMountPoint{
			Source: "/dev/shm",
			Target: filepath.Join(root, "newroot", "dev", "shm"),
		},
		ReadOnly: false,
		IsDir:    true,
		Flags:    syscall.MS_NOEXEC | syscall.MS_NOSUID,
	})

	// bind mount /dev/console if needed
	ttyname := strings.TrimSpace(os.Getenv("SKEWER_TTYNAME"))
	if len(ttyname) > 0 {
		bindMounts = append(bindMounts, bindMountPoint{
			baseMountPoint: baseMountPoint{
				Source: ttyname,
				Target: filepath.Join(root, "newroot", "dev", "console"),
			},
			ReadOnly: false,
			IsDir:    false,
			Flags:    syscall.MS_NOSUID | syscall.MS_NOEXEC,
		})
	}

	// bind mount the skewer executable
	bindMounts = append(bindMounts, bindMountPoint{
		baseMountPoint: baseMountPoint{
			Source: targetExec,
			Target: filepath.Join(root, "newroot", targetExec),
		},
		ReadOnly: true,
		IsDir:    false,
		Flags:    syscall.MS_NOSUID | syscall.MS_NODEV,
	})

	// RW bind-mount the Store if needed
	storePath := strings.TrimSpace(os.Getenv("SKEWER_STORE_PATH"))
	if len(storePath) > 0 {
		bindMounts = append(bindMounts, bindMountPoint{
			baseMountPoint: baseMountPoint{
				Source: storePath,
				Target: filepath.Join(root, "newroot", "tmp", "store", storePath),
			},
			ReadOnly: false,
			IsDir:    true,
			Flags:    syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV,
		})
	}

	// RW bind-mount the directory for file destination if needed
	fileDestDir := strings.TrimSpace(os.Getenv("SKEWER_FILEDEST_DIR"))
	if len(fileDestDir) > 0 {
		bindMounts = append(bindMounts, bindMountPoint{
			baseMountPoint: baseMountPoint{
				Source: fileDestDir,
				Target: filepath.Join(root, "newroot", "tmp", "filedest", fileDestDir),
			},
			ReadOnly: false,
			IsDir:    true,
			Flags:    syscall.MS_NOEXEC | syscall.MS_NODEV | syscall.MS_NOSUID,
		})
	}

	// mount SKEWER_CERT_FILES
	certFiles = filepath.SplitList(os.Getenv("SKEWER_CERT_FILES"))
	if len(certFiles) > 0 {
		for _, certFile := range certFiles {
			if len(certFile) == 0 {
				continue
			}
			bindMounts = append(bindMounts, bindMountPoint{
				baseMountPoint: baseMountPoint{
					Source: certFile,
					Target: filepath.Join(root, "newroot", "tmp", "certfiles", certFile),
				},
				ReadOnly: true,
				IsDir:    false,
				Flags:    syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV,
			})
		}
	}

	// mount SKEWER_CERT_PATHS directories
	certPaths = filepath.SplitList(os.Getenv("SKEWER_CERT_PATHS"))
	if len(certPaths) > 0 {
		for _, certPath := range certPaths {
			if len(certPath) == 0 {
				continue
			}
			bindMounts = append(bindMounts, bindMountPoint{
				baseMountPoint: baseMountPoint{
					Source: certPath,
					Target: filepath.Join(root, "newroot", "tmp", "certpaths", certPath),
				},
				ReadOnly: true,
				IsDir:    true,
				Flags:    syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV,
			})
		}
	}

	// mount the directories we have to poll
	polldirs := filepath.SplitList(os.Getenv("SKEWER_POLLDIRS"))
	if len(polldirs) > 0 {
		for _, pdir := range polldirs {
			if len(pdir) == 0 {
				continue
			}
			bindMounts = append(bindMounts, bindMountPoint{
				baseMountPoint: baseMountPoint{
					Source: pdir,
					Target: filepath.Join(root, "newroot", "tmp", "polldirs", pdir),
				},
				ReadOnly: true,
				IsDir:    true,
				Flags:    syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV,
			})
		}
	}

	for _, mountPoint := range bindMounts {
		if mounted.Has(mountPoint.Source) {
			continue
		}
		if mountPoint.IsDir {
			if !utils.IsDir(mountPoint.Source) {
				return "", fmt.Errorf("mount source '%s' is not a directory", mountPoint.Source)
			}
			os.MkdirAll(mountPoint.Target, 0755)
			flags := mountPoint.Flags | syscall.MS_REC | syscall.MS_BIND
			if mountPoint.ReadOnly {
				flags = flags | syscall.MS_RDONLY
			}
			err := syscall.Mount(
				mountPoint.Source,
				mountPoint.Target,
				"bind",
				flags,
				"",
			)
			if err != nil {
				return "", fmt.Errorf("Error binding '%s' to '%s': %s", mountPoint.Source, mountPoint.Target, err)
			}
			flags = flags | syscall.MS_REMOUNT
			err = syscall.Mount(
				mountPoint.Source,
				mountPoint.Target,
				"bind",
				flags,
				"",
			)
			if err != nil {
				return "", fmt.Errorf("Error binding '%s' to '%s': %s", mountPoint.Source, mountPoint.Target, err)
			}
			mounted.Add(baseMountPoint{
				Source: mountPoint.Source,
				Target: mountPoint.Target,
			})
		} else {
			if !utils.FileExists(mountPoint.Source) {
				return "", fmt.Errorf("mount source '%s' (to '%s') does not exist", mountPoint.Source, mountPoint.Target)
			}
			os.MkdirAll(filepath.Dir(mountPoint.Target), 0755)
			f, err := os.Create(mountPoint.Target)
			if err != nil {
				return "", fmt.Errorf("Error creating '%s' in chroot: %s", mountPoint.Source, err)
			}
			f.Close()
			flags := mountPoint.Flags | syscall.MS_BIND
			if mountPoint.ReadOnly {
				flags = flags | syscall.MS_RDONLY
			}
			err = syscall.Mount(
				mountPoint.Source,
				mountPoint.Target,
				"bind",
				flags,
				"",
			)
			if err != nil {
				return "", fmt.Errorf("Error binding '%s' to '%s': %s", mountPoint.Source, mountPoint.Target, err)
			}
			flags = flags | syscall.MS_REMOUNT
			err = syscall.Mount(
				mountPoint.Source,
				mountPoint.Target,
				"bind",
				flags,
				"",
			)
			if err != nil {
				return "", fmt.Errorf("Error binding '%s' to '%s': %s", mountPoint.Source, mountPoint.Target, err)
			}
			mounted.Add(baseMountPoint{
				Source: mountPoint.Source,
				Target: mountPoint.Target,
			})
		}
	}

	// we have mounted everything we wanted in /lib, we can remount it read-only
	syscall.Mount(
		"tmpfs",
		filepath.Join(root, "newroot", "lib"),
		"tmpfs",
		syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_REMOUNT|syscall.MS_RDONLY,
		"mode=755",
	)
	syscall.Mount(
		"tmpfs",
		filepath.Join(root, "newroot", "lib64"),
		"tmpfs",
		syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_REMOUNT|syscall.MS_RDONLY,
		"mode=755",
	)

	return root, nil
}
