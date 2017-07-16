// +build !linux

package sys

type NotLinuxError struct{}

func (e NotLinuxError) Error() string {
	return "Only available on Linux"
}

func SetNonDumpable() error {
	return NotLinuxError{}
}
