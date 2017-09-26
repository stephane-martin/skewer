// +build !linux

package dumpable

func SetNonDumpable() error {
	return nil
}

func SetDumpable() error {
	return nil
}

