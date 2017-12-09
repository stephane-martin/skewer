package namespaces

//go:generate goderive .

import (
	"os"
	"path/filepath"
	"strings"
)

var hiddens []string = []string{
	"/lib/modules", "/lib/apparmor", "/lib/firmware", "/lib/modprobe.d", "/lib/recovery-mode", "/lib/udev", "/lib/brltty", "/lib/cgmanager",
	"/lib/crda", "/lib/cryptsetup", "/lib/hdparm", "/lib/ifupdown", "/lib/init", "/lib/linux-sound-base", "/lib/ufw", "/lib/xtables",
	"/lib/security",
}

func myWalk(root string) []string {

	shared_libs := map[string]bool{}

	walk := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if strings.Contains(path, "/security/") {
				return filepath.SkipDir
			}
			if deriveAnyHidden(func(hidden string) bool { return strings.HasPrefix(path, hidden) }, hiddens) {
				return filepath.SkipDir
			}
			return nil
		} else {
			if strings.Contains(path, "/security/") {
				return nil
			}
			if deriveAnyHidden(func(hidden string) bool { return strings.HasPrefix(path, hidden) }, hiddens) {
				return nil
			}
			if strings.HasSuffix(path, ".so") || strings.Contains(path, ".so.") {
				shared_libs[path] = true
			}
			return nil
		}
	}
	_ = filepath.Walk(root, walk)
	return deriveKeys(shared_libs)

}
