// +build linux

package namespaces

import (
	"fmt"
	"strings"

	"github.com/EricLagergren/go-gnulib/ttyname"
)

func setupEnv(storePath string, confDir string) (env []string) {
	env = []string{}
	if ttyname.IsAtty(1) {
		myTtyName, _ := ttyname.TtyName(1)
		env = append(env, fmt.Sprintf("SKEWER_TTYNAME=%s", myTtyName))
	}

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
