// +build linux

package sys


import "golang.org/x/sys/unix"
import "net"


func GetCredentials(conn *net.UnixConn) (int, int, int, error) {
	f, err := conn.File()
	if err != nil {
		return 0, 0, 0, err
	}
	creds, err := unix.GetsockoptUcred(int(f), unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return 0, 0, 0, err
	}
	return creds.Pid, creds.Uid, creds.Gid, nil
}


