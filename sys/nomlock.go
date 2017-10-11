// +build darwin netbsd

var MlockSupported bool = false

func MlockAll() error {
	return nil
}
