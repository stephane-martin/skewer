// +build linux

package sys

import "syscall"

func SetHostname(name string) error {
	return syscall.Sethostname([]byte(name))
}
