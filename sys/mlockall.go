// +build linux

package sys

import "syscall"
import "golang.org/x/sys/unix"

func MlockAll() error {
	return unix.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE)
}
