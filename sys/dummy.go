// +build !linux

package sys

func SetNonDumpable() error {
	return nil
}
