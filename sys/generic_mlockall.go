package sys

var mlockSupported bool

type NotUnixError struct{}

func (e NotUnixError) Error() string {
	return "MlockAll not available"
}

func MlockAll() error {
	if mlockSupported {
		return mlockall()
	} else {
		return NotUnixError{}
	}
}
