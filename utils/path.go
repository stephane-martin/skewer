package utils

import "os"

func IsDir(path string) bool {
	if infos, err := os.Stat(path); err == nil {
		return infos.IsDir()
	}
	return false
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
