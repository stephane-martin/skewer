package utils

import (
	"io/ioutil"
	"os"
)

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

func LsDir(path string) (res []string) {
	res = make([]string, 0)
	entries, err := ioutil.ReadDir(path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		res = append(res, entry.Name())
	}
	return
}
