// +build !linux

package sys

type NotLinuxError struct{}

func (e NotLinuxError) Error() string {
	return "Only available on Linux"
}

func SetHostname(name string) error {
	return nil
}

func GetTick() int {
	return 0
}
