// +build dragonfly freebsd linux openbsd solaris

package sys

import "syscall"
import "golang.org/x/sys/unix"

func init() {
	mlockSupported = true
}

func mlockall() error {
	return unix.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE)
}
