// +build linux

package sys

/*
#include <sys/types.h>
#include <unistd.h>
*/
import "C"
import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"

	"github.com/syndtr/gocapability/capability"
	"golang.org/x/sys/unix"
)

var CAPS_TO_KEEP []capability.Cap
var CapabilitiesSupported bool = true

func init() {
	CAPS_TO_KEEP = []capability.Cap{
		capability.CAP_NET_BIND_SERVICE,
		capability.CAP_AUDIT_READ,
		capability.CAP_AUDIT_CONTROL,
		capability.CAP_IPC_LOCK,
	}
}

func Setuid(uid int) {
	C.setuid(C.__uid_t(uid))
}

func Setgid(gid int) {
	C.setgid(C.__gid_t(gid))
}

func NoNewPriv() error {
	return unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
}

func KeepCaps() error {
	return unix.Prctl(unix.PR_SET_SECUREBITS, uintptr(5)|uintptr(3), 0, 0, 0)
}

func Predrop() error {
	c, err := NewCapabilitiesQuery()
	if err != nil {
		return err
	}
	toKeepMap := map[capability.Cap]bool{}
	for _, cap := range CAPS_TO_KEEP {
		toKeepMap[cap] = true
	}
	toKeepMap[capability.CAP_SETUID] = true
	toKeepMap[capability.CAP_SETGID] = true
	toKeepMap[capability.CAP_SETPCAP] = true

	for i := 0; i <= int(capability.CAP_LAST_CAP); i++ {
		if toKeepMap[capability.Cap(i)] {
			continue
		}
		if c.caps.Get(capability.EFFECTIVE, capability.Cap(i)) {
			c.caps.Unset(capability.EFFECTIVE, capability.Cap(i))
		}
		if c.caps.Get(capability.PERMITTED, capability.Cap(i)) {
			c.caps.Unset(capability.PERMITTED, capability.Cap(i))
		}
		if c.caps.Get(capability.BOUNDING, capability.Cap(i)) {
			c.caps.Unset(capability.BOUNDING, capability.Cap(i))
		}
	}

	err = c.caps.Apply(capability.BOUNDING)
	if err != nil {
		return err
	}

	return c.caps.Apply(capability.CAPS)
}

func FixLinuxPrivileges(uid string, gid string) error {

	numuid, numgid, err := LookupUid(uid, gid)
	if err != nil {
		return err
	}

	c, err := NewCapabilitiesQuery()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Starting with capabilities: %s\n", c.caps.StringCap(capability.EFFECTIVE))
	if numuid == os.Getuid() && numgid == os.Getgid() && !c.NeedDrop() {
		return nil
	}
	return Drop(numuid, numgid)
}

func CanReadAuditLogs() bool {
	c, err := NewCapabilitiesQuery()
	if err != nil {
		return false
	}
	return c.CanReadAuditLogs()
}

type CapabilitiesQuery struct {
	caps capability.Capabilities
}

func NewCapabilitiesQuery() (*CapabilitiesQuery, error) {
	caps, err := capability.NewPid(os.Getpid())
	if err != nil {
		return nil, err
	}
	return &CapabilitiesQuery{caps: caps}, nil
}

func (c *CapabilitiesQuery) NeedDrop() bool {
	toKeepMap := map[capability.Cap]bool{}
	for _, cap := range CAPS_TO_KEEP {
		toKeepMap[cap] = true
	}
	for i := 0; i <= int(capability.CAP_LAST_CAP); i++ {
		if toKeepMap[capability.Cap(i)] {
			continue
		}
		if c.caps.Get(capability.EFFECTIVE, capability.Cap(i)) {
			return true
		}
		if c.caps.Get(capability.INHERITABLE, capability.Cap(i)) {
			return true
		}
		if c.caps.Get(capability.PERMITTED, capability.Cap(i)) {
			return true
		}
	}
	return false
}

func Drop(uid int, gid int) error {
	c, err := NewCapabilitiesQuery()
	if err != nil {
		return err
	}

	curUid := os.Getuid()
	curGid := os.Getgid()
	if (curUid != uid || curGid != gid) && !c.CanChangeUid() {
		return fmt.Errorf("Can't change UID or GID")
	}

	if curUid == uid && curGid == gid {
		// just have to drop the superfluous caps
		toKeepMap := map[capability.Cap]bool{}
		for _, cap := range CAPS_TO_KEEP {
			toKeepMap[cap] = true
		}

		for i := 0; i <= int(capability.CAP_LAST_CAP); i++ {
			if toKeepMap[capability.Cap(i)] {
				continue
			}
			if c.caps.Get(capability.EFFECTIVE, capability.Cap(i)) {
				c.caps.Unset(capability.EFFECTIVE, capability.Cap(i))
			}
			if c.caps.Get(capability.PERMITTED, capability.Cap(i)) {
				c.caps.Unset(capability.PERMITTED, capability.Cap(i))
			}
			if c.caps.Get(capability.INHERITABLE, capability.Cap(i)) {
				c.caps.Unset(capability.INHERITABLE, capability.Cap(i))
			}
		}

		c.caps.Clear(capability.AMBIENT)

		err := c.caps.Apply(capability.AMBIENT)
		if err != nil {
			return err
		}
		err = c.caps.Apply(capability.CAPS)
		if err != nil {
			return err
		}
		err = NoNewPriv()
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Now running with capabilities: %s\n", c.caps.StringCap(capability.EFFECTIVE))

		return nil
	} else if !c.CanModifySecurebits() {
		return fmt.Errorf("Asked to change UID/GID, but no way to set the correct capabilities")
	} else {

		err := KeepCaps()
		if err != nil {
			return err
		}
		os.Stdout.Chown(uid, gid)
		os.Stdin.Chown(uid, gid)
		os.Stderr.Chown(uid, gid)

		/*
			err = NoNewPriv()
			if err != nil {
				return err
			}
		*/
		err = Predrop()
		if err != nil {
			return err
		}

		c, err = NewCapabilitiesQuery()
		if err != nil {
			return err
		}

		Setgid(gid)
		Setuid(uid)

		// add the group adm to the supplementary groups
		admGroup, err := user.LookupGroup("adm")
		if err == nil {
			numAdmGroup, err := strconv.Atoi(admGroup.Gid)
			if err != nil {
				return err
			}
			err = unix.Setgroups([]int{numAdmGroup})
			if err != nil {
				return err
			}
		}

		c.caps.Unset(capability.CAPS|capability.BOUNDING, capability.CAP_SETUID, capability.CAP_SETGID, capability.CAP_SETPCAP)
		err = c.caps.Apply(capability.BOUNDING)
		if err != nil {
			return err
		}
		err = c.caps.Apply(capability.CAPS)
		if err != nil {
			return err
		}

		c.caps.Clear(capability.AMBIENT)
		c.caps.Clear(capability.INHERITABLE)

		for i := 0; i <= int(capability.CAP_LAST_CAP); i++ {
			if c.caps.Get(capability.PERMITTED, capability.Cap(i)) {
				c.caps.Set(capability.CAPS, capability.Cap(i))
				c.caps.Set(capability.AMBIENT, capability.Cap(i))
			}
		}

		err = c.caps.Apply(capability.CAPS)
		if err != nil {
			return err
		}

		err = c.caps.Apply(capability.AMBIENT)
		if err != nil {
			return err
		}

		exe, err := os.Executable()
		if err != nil {
			return err
		}

		cmd := exec.Cmd{
			Args:   os.Args,
			Path:   exe,
			Stdin:  nil,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
		err = cmd.Start()
		if err != nil {
			return err
		}
		NoNewPriv()
		signal.Ignore(syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
		cmd.Process.Wait()
		os.Exit(0)
		return nil
	}

}

func (c *CapabilitiesQuery) NeedMore() bool {
	for _, cap := range CAPS_TO_KEEP {
		if !c.caps.Get(capability.EFFECTIVE, cap) {
			return true
		}
	}
	return false
}

func (c *CapabilitiesQuery) CanModifySecurebits() bool {
	return c.caps.Get(capability.EFFECTIVE, capability.CAP_SETPCAP)
}

func (c *CapabilitiesQuery) CanChangeUid() bool {
	return c.caps.Get(capability.EFFECTIVE, capability.CAP_SETUID) && c.caps.Get(capability.EFFECTIVE, capability.CAP_SETGID)
}

func (c *CapabilitiesQuery) CanReadAuditLogs() bool {
	return c.caps.Get(capability.EFFECTIVE, capability.CAP_AUDIT_READ) && c.caps.Get(capability.EFFECTIVE, capability.CAP_AUDIT_CONTROL)
}
