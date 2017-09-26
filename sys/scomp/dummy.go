// +build !linux

package scomp

func SetupSeccomp(name string) (err error) {
	return nil
}
