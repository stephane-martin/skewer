// +build openbsd

package scomp

import "golang.org/x/sys/unix"

var PledgeSupported bool = true

func SetupPledge(name string) (err error) {
	switch name {
	case "skewer-tcp", "skewer-udp", "skewer-relp", "skewer-conf":
		err = unix.Pledge("stdio rpath flock dns sendfd recvfd ps inet unix", nil)
	case "skewer-store":
		err = unix.Pledge("stdio rpath flock dns sendfd recvfd ps inet unix wpath cpath tmppath fattr chown", nil)
	default:
		err = unix.Pledge("mcast stdio rpath flock dns sendfd recvfd ps inet unix wpath cpath tmppath fattr chown getpw tty proc exec id", nil)
	}
	return
}

