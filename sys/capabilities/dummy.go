// +build !linux

package capabilities

var CapabilitiesSupported bool = false

func NeedFixLinuxPrivileges(uid, gid string) (bool, error) {
	return false, nil
}

func FixLinuxPrivileges(uid string, gid string) error {
	return nil
}

func DropNetBind() error {
	return nil
}

func GetCaps() string {
	return ""
}

func Predrop() error {
	return nil
}

func NoNewPriv() error {
	return nil
}
