// +build darwin netbsd

package sys

var MlockSupported bool = false

func MlockAll() error {
	return nil
}
