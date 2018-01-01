// +build openbsd

package scomp

import (
	"github.com/stephane-martin/skewer/services/base"
	"golang.org/x/sys/unix"
)

var PledgeSupported bool = true

//SetupPledge actually runs the pledge syscall based on the process name
func SetupPledge(t base.Types) (err error) {
	switch t {
	case base.TCP,
		base.UDP,
		base.Graylog,
		base.RELP,
		base.DirectRELP,
		base.Configuration,
		base.Accounting,
		base.KafkaSource:

		err = unix.Pledge("stdio rpath flock dns sendfd recvfd ps inet unix getpw", nil)

	case base.Store:
		err = unix.Pledge("stdio rpath flock dns sendfd recvfd ps inet unix getpw wpath cpath tmppath fattr chown", nil)

	default:
		err = unix.Pledge("mcast stdio rpath flock dns sendfd recvfd ps inet unix wpath cpath tmppath fattr chown getpw tty proc exec id", nil)
	}
	return
}
