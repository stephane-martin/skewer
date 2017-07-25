package sys

var MlockSupported bool

type NotUnixError struct{}

func (e NotUnixError) Error() string {
	return "MlockAll not available"
}

func MlockAll() error {
	if MlockSupported {
		return mlockall()
	} else {
		return NotUnixError{}
	}
}
