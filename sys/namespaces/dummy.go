// +build !linux

package namespaces

import (
	"fmt"
	"os/exec"
	"strings"
)

func StartInNamespaces(command *exec.Cmd, dumpable bool, storePath string, confDir string) error {
	command.Env = append(command.Env, setupEnv(storePath, confDir)...)
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

func setupEnv(storePath string, confDir string) (env []string) {
	env = []string{}

	confDir = strings.TrimSpace(confDir)
	if len(confDir) > 0 {
		env = append(env, fmt.Sprintf("SKEWER_CONF_DIR=%s", confDir))
	}

	storePath = strings.TrimSpace(storePath)
	if len(storePath) > 0 {
		env = append(env, fmt.Sprintf("SKEWER_STORE_PATH=%s", storePath))
	}
	return env
}
