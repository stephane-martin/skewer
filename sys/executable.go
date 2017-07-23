// +build !openbsd

package sys

import "os"

func Executable() (string, error) {
	return os.Executable()
}
