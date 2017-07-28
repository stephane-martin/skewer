package sys

import (
	"syscall"
)

func SocketPair(typ int) (int, int, error) {
	var fds [2]int
	var err error
	fds, err = syscall.Socketpair(syscall.AF_LOCAL, typ, 0)
	if err != nil {
		return 0, 0, err
	}
	return fds[0], fds[1], nil
}
