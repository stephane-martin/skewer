// +build linux

package sys

import "golang.org/x/sys/unix"

func SetNonDumpable() error {
	return unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}
