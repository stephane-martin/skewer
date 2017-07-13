// +build !linux

package sys

import "net"

type NotLinuxError struct{}

func (e NotLinuxError) Error() string {
	return "Only available on Linux"
}

func SetNonDumpable() error {
	return NotLinuxError{}
}

func GetCredentials(conn *net.UnixConn) (int32, uint32, uint32, error) {
	return 0, 0, 0, NotLinuxError{}
}

func MlockAll() error {
	return NotLinuxError{}
}
