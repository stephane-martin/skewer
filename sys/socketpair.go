package sys

import (
	"syscall"
)

func SocketPair() (int, int, error) {
	var fds [2]int
	var err error
	fds, err = syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)
	if err != nil {
		return 0, 0, err
	}
	return fds[0], fds[1], nil
}
