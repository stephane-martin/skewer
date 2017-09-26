// +build !linux

package namespaces

import "os/exec"

func StartInNamespaces(command *exec.Cmd, dumpable bool, storePath string, confDir string) error {
	command.Env = append(command.Env, setupEnv(storePath, confDir)...)
	// TODO: DUMPABLE should only be set in the child, just to write uid_map
	if !dumpable {
		SetDumpable()
	}
	err := command.Start()
	if !dumpable {
		SetNonDumpable()
	}
	return err
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
