// +build openbsd

package scomp

import (
	"github.com/stephane-martin/skewer/services"
	"golang.org/x/sys/unix"
)

var PledgeSupported bool = true

//SetupPledge actually runs the pledge syscall based on the process name
func SetupPledge(name string) (err error) {
	switch name {
	case services.Types2Names[services.TCP],
		services.Types2Names[services.UDP],
		services.Types2Names[services.Graylog],
		services.Types2Names[services.RELP],
		services.Types2Names[services.DirectRELP],
		services.Types2Names[services.Configuration],
		services.Types2Names[services.Accounting],
		services.Types2Names[services.KafkaSource]:

		err = unix.Pledge("stdio rpath flock dns sendfd recvfd ps inet unix getpw", nil)

	case services.Types2Names[services.Store]:
		err = unix.Pledge("stdio rpath flock dns sendfd recvfd ps inet unix getpw wpath cpath tmppath fattr chown", nil)

	default:
		err = unix.Pledge("mcast stdio rpath flock dns sendfd recvfd ps inet unix wpath cpath tmppath fattr chown getpw tty proc exec id", nil)
	}
	return
}
