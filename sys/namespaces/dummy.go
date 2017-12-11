// +build !linux

package namespaces

import (
	"os/exec"
)

func StartInNamespaces(command *exec.Cmd, dumpable bool, storePath, confDir, acctPath, fileDestPath string) error {
	return command.Start()
}

func PivotRoot(root string) (err error) {
	return nil
}

func SetJournalFs(targetExec string) error {
	return nil
}

func MakeChroot(targetExec string) (string, error) {
	return "", nil
}
