// +build linux

package sys

import "golang.org/x/sys/unix"
import "net"

func GetCredentials(conn *net.UnixConn) (int32, uint32, uint32, error) {
	f, err := conn.File()
	if err != nil {
		return 0, 0, 0, err
	}
	creds, err := unix.GetsockoptUcred(int(f.Fd()), unix.SOL_SOCKET, unix.SO_PEERCRED)
	f.Close()
	if err != nil {
		return 0, 0, 0, err
	}
	return creds.Pid, creds.Uid, creds.Gid, nil
}
