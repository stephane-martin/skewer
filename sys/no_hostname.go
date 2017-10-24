// +build !linux

package sys

func SetHostname(name string) error {
	return nil
}
