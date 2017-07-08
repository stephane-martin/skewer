// +build !linux

package sys

import "net"

type NotLinuxError struct{}

func (e NotLinuxError) Error() string {
	return "GetCredentials is only available in Linux"
}

func SetNonDumpable() error {
	return nil
}

func GetCredentials(conn *net.UnixConn) (int32, uint32, uint32, error) {
	return 0, 0, 0, NotLinuxError{}
}
