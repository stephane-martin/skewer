// +build !linux

package sys

type NotLinuxError struct{}


func (e NotLinuxError) Error() string {
	return "Only available on Linux"
}


