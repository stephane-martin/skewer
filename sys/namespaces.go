// +build linux

package sys

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/EricLagergren/go-gnulib/ttyname"
	"github.com/stephane-martin/skewer/utils"
)

type mountPoint struct {
	Source string
	Target string
	Fs     string
	Flags  int
	Data   string
}

func StartInNamespaces(command *exec.Cmd, dumpable bool, storePath string, confDir string) error {
	myTtyName := ""
	if ttyname.IsAtty(1) {
		myTtyName, _ = ttyname.TtyName(1)
		command.Env = append(command.Env, fmt.Sprintf("SKEWER_TTYNAME=%s", myTtyName))
	}

	confDir = strings.TrimSpace(confDir)
	if len(confDir) > 0 {
		command.Env = append(command.Env, fmt.Sprintf("SKEWER_CONF_DIR=%s", confDir))
	}

	storePath = strings.TrimSpace(storePath)
	if len(storePath) > 0 {
		command.Env = append(command.Env, fmt.Sprintf("SKEWER_STORE_PATH=%s", storePath))
	}

	command.SysProcAttr = &syscall.SysProcAttr{
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
	}

	// TODO: DUMPABLE should only be set in the child, just to write uid_map
	if !dumpable {
		SetDumpable()
	}
	err := command.Start()
	if !dumpable {
		SetNonDumpable()
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
			Target: "/bin",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			Target: "/sbin",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			Target: "/lib",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			Target: "/lib64",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			Target: "/usr",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			Target: "/var",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC,
		},
		{
			Target: "/home",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
		},
		{
			Target: "/etc",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC,
		},
	}

	mounts := []mountPoint{
		{
			Source: "proc",
			Target: "/proc",
			Fs:     "proc",
			Flags:  syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV,
		},
		{
			Source: "tmpfs",
			Target: "/dev",
			Fs:     "tmpfs",
			Flags:  syscall.MS_NOSUID | syscall.MS_NOEXEC,
			Data:   "mode=755",
		},
		{
			Source: "tmpfs",
			Target: "/boot",
			Fs:     "tmpfs",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC,
			Data:   "mode=700",
		},
	}

	temp, err := ioutil.TempDir("", "skewer-confined")
	if err != nil {
		return fmt.Errorf("TempDir error: %s", err)
	}
	err = syscall.Mount("/dev", temp, "bind", syscall.MS_BIND|syscall.MS_REC, "")
	if err != nil {
		return fmt.Errorf("Error bind-mounting /dev: %s", err)
	}

	for _, m := range mounts {
		if _, err := os.Stat(m.Target); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(m.Target, 0755)
				if err != nil {
					return fmt.Errorf("mkdirall %s error: %v", m.Target, err)
				}
			}
		}
		err = syscall.Mount(m.Source, m.Target, m.Fs, uintptr(m.Flags), "")
		if err != nil {
			return fmt.Errorf("failed to mount %s: %s", m.Target, err.Error())
		}
	}

	for _, m := range roRemounts {
		if _, err = os.Stat(m.Target); err == nil {
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
	devices := []string{"null", "zero", "full", "random", "urandom"}
	for _, device := range devices {
		f, err := os.Create(filepath.Join("/dev", device))
		if err == nil {
			f.Close()
			err = syscall.Mount(filepath.Join(temp, device), filepath.Join("/dev", device), "bind", syscall.MS_BIND, "")
			if err != nil {
				return fmt.Errorf("Error mounting device %s: %s", device, err)
			}
		}
	}
	err = syscall.Unmount(temp, syscall.MNT_DETACH)
	if err != nil {
		return fmt.Errorf("Error unmounting %s: %s", temp, err)
	}
	err = os.Remove(temp)
	if err != nil {
		return fmt.Errorf("Error removing %s: %s", temp, err)
	}
	err = syscall.Mount("tmpfs", "/tmp", "tmpfs", syscall.MS_NODEV|syscall.MS_NOEXEC|syscall.MS_NOSUID, "")
	if err != nil {
		return fmt.Errorf("Error mounting /tmp: %s", err)
	}

	return nil
}

func MakeChroot(targetExec string) (string, error) {
	systemMountsMap := map[string]bool{
		//"/etc",
		//"/var",
		"/bin": true, // TODO: remove (systemctl in path to detect if we should collect from journald...)
		//"/usr/bin":  true,
		//"/sbin":     true,
		//"/usr/sbin": true,
	}

	confDir := strings.TrimSpace(os.Getenv("SKEWER_CONF_DIR"))
	if len(confDir) > 0 {
		systemMountsMap[confDir] = true
	}

	systemMounts := make([]string, 0, len(systemMountsMap))
	for dir, b := range systemMountsMap {
		if b {
			systemMounts = append(systemMounts, dir)
		}
	}

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
			Source: "proc",
			Target: "/proc",
			Fs:     "proc",
			Flags:  syscall.MS_NOSUID | syscall.MS_NOEXEC | syscall.MS_NODEV,
		},
		{
			Source: "tmpfs",
			Target: "/dev",
			Fs:     "tmpfs",
			Flags:  syscall.MS_NOSUID | syscall.MS_NOEXEC,
			Data:   "mode=755",
		},
		{
			Source: "tmpfs",
			Target: "/run",
			Fs:     "tmpfs",
			Flags:  syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOSUID,
			Data:   "mode=755",
		},
		{
			Source: "devpts",
			Target: "/dev/pts",
			Fs:     "devpts",
			Flags:  syscall.MS_NOSUID | syscall.MS_NOEXEC,
			Data:   "newinstance,ptmxmode=0666,mode=620",
		},
		{
			Source: "tmpfs",
			Target: "/tmp",
			Fs:     "tmpfs",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
			Data:   "mode=700",
		},
		{
			Source: "tmpfs",
			Target: "/lib",
			Fs:     "tmpfs",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
			Data:   "mode=755",
		},
		{
			Source: "tmpfs",
			Target: "/lib64",
			Fs:     "tmpfs",
			Flags:  syscall.MS_NOSUID | syscall.MS_NODEV,
			Data:   "mode=755",
		},
	}

	for _, systemMount := range systemMounts {
		if _, err := os.Stat(systemMount); err == nil {
			mounts = append(mounts, mountPoint{
				Source: systemMount,
				Target: systemMount,
				Fs:     "bind",
				Flags:  syscall.MS_BIND | syscall.MS_REC | syscall.MS_RDONLY | syscall.MS_NODEV | syscall.MS_NOSUID,
			})
			mounts = append(mounts, mountPoint{
				Source: systemMount,
				Target: systemMount,
				Fs:     "bind",
				Flags:  syscall.MS_BIND | syscall.MS_REC | syscall.MS_RDONLY | syscall.MS_NODEV | syscall.MS_NOSUID | syscall.MS_REMOUNT,
			})
		}
	}

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
	}

	// bind mount shared libraries from /lib and /lib64
	// TODO: optimize the search for shared libs

	hiddens := []string{
		"/lib/modules", "/lib/apparmor", "/lib/firmware", "/lib/modprobe.d", "/lib/recovery-mode", "/lib/udev", "/lib/brltty", "/lib/cgmanager",
		"/lib/crda", "/lib/cryptsetup", "/lib/hdparm", "/lib/ifupdown", "/lib/init", "/lib/linux-sound-base", "/lib/ufw", "/lib/xtables",
		"/lib/security",
	}

	shared_libs := []string{}

	if _, err := os.Stat("/lib"); err == nil {
		filepath.Walk("/lib", func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			if strings.Contains(path, "/security/") {
				return nil
			}
			for _, prefix := range hiddens {
				if strings.HasPrefix(path, prefix) {
					return nil
				}
			}
			if strings.HasSuffix(path, ".so") || strings.Contains(path, ".so.") {
				shared_libs = append(shared_libs, path)
			}
			return nil
		})
	}

	if _, err := os.Stat("/lib64"); err == nil {
		filepath.Walk("/lib64", func(path string, info os.FileInfo, err error) error {
			for _, prefix := range hiddens {
				if info.IsDir() {
					return nil
				}
				if strings.HasPrefix(path, prefix) {
					return nil
				}
			}

			if strings.HasSuffix(path, ".so") || strings.Contains(path, ".so.") {
				shared_libs = append(shared_libs, path)
			}
			return nil
		})
	}

	for _, library := range shared_libs {
		libraryDir := filepath.Dir(library)
		targetDir := filepath.Join(root, "newroot", libraryDir)
		os.MkdirAll(targetDir, 0755)
		target := filepath.Join(root, "newroot", library)
		f, err := os.Create(target)
		if err == nil {
			f.Close()
			syscall.Mount(library, target, "bind", syscall.MS_BIND|syscall.MS_RDONLY|syscall.MS_NODEV|syscall.MS_NOSUID, "")
			syscall.Mount(library, target, "bind", syscall.MS_BIND|syscall.MS_RDONLY|syscall.MS_NODEV|syscall.MS_NOSUID|syscall.MS_REMOUNT, "")

		}
	}

	// we have mounted everything we wanted in /lib, we can remount it read-only
	if _, err := os.Stat("/lib"); err == nil {
		syscall.Mount("/lib", filepath.Join(root, "newroot", "lib"), "bind", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_REMOUNT|syscall.MS_RDONLY, "mode=755")
	}
	if _, err := os.Stat("/lib64"); err == nil {
		syscall.Mount("/lib64", filepath.Join(root, "newroot", "lib64"), "bind", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_REMOUNT|syscall.MS_RDONLY, "mode=755")
	}

	// bind mount the skewer executable
	executableDir := filepath.Dir(targetExec)
	targetDir := filepath.Join(root, "newroot", executableDir)
	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return "", fmt.Errorf("mkdirall %s error: %v", targetDir, err)
	}
	target := filepath.Join(root, "newroot", targetExec)
	f, err := os.Create(target)
	if err == nil {
		f.Close()
		err = syscall.Mount(targetExec, target, "bind", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_RDONLY, "")
		if err != nil {
			return "", fmt.Errorf("failed to mount %s to %s: %s", targetExec, target, err.Error())
		}
		err = syscall.Mount(targetExec, target, "bind", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_REMOUNT|syscall.MS_RDONLY, "")
		if err != nil {
			return "", fmt.Errorf("failed to remount readonly %s to %s: %s", targetExec, target, err.Error())
		}
	} else {
		return "", fmt.Errorf("failed to create %s: %s", target, err.Error())
	}

	// bind mount some devices in /dev
	devices := []string{"null", "zero", "full", "random", "urandom", "tty"}

	for _, device := range devices {
		source := filepath.Join("/dev", device)
		target := filepath.Join(root, "newroot", "dev", device)
		f, err := os.Create(target)
		if err == nil {
			f.Close()
			err = syscall.Mount(source, target, "bind", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_NOEXEC, "")
			err = syscall.Mount(source, target, "bind", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_REMOUNT, "")
			if err != nil {
				return "", fmt.Errorf("failed to mount %s to %s: %v", source, target, err)
			}
		} else {
			return "", fmt.Errorf("failed to create %s: %s", target, err.Error())
		}
	}

	// bind mount /dev/console if needed
	os.Mkdir(filepath.Join(root, "newroot", "dev", "shm"), 0755)
	ttyname := strings.TrimSpace(os.Getenv("SKEWER_TTYNAME"))
	if len(ttyname) > 0 {
		target := filepath.Join(root, "newroot", "dev", "console")
		f, err := os.Create(target)
		if err == nil {
			f.Close()
			syscall.Mount(ttyname, target, "bind", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_NOEXEC, "")
			syscall.Mount(ttyname, target, "bind", syscall.MS_BIND|syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_REMOUNT, "")
		}
	}

	// bind mount the Store if needed
	storePath := strings.TrimSpace(os.Getenv("SKEWER_STORE_PATH"))
	if len(storePath) > 0 {
		if infos, err := os.Stat(storePath); err == nil {
			if infos.IsDir() {
				storePath, _ = filepath.Abs(storePath)
				target := filepath.Join(root, "newroot", storePath)
				os.MkdirAll(target, 0755)
				syscall.Mount(
					storePath, target, "bind",
					syscall.MS_BIND|syscall.MS_REC|syscall.MS_NOEXEC|syscall.MS_NODEV|syscall.MS_NOSUID,
					"",
				)
				syscall.Mount(
					storePath, target, "bind",
					syscall.MS_BIND|syscall.MS_REC|syscall.MS_NOEXEC|syscall.MS_NODEV|syscall.MS_NOSUID|syscall.MS_REMOUNT,
					"",
				)
			}
		}
	}

	return root, nil
}
