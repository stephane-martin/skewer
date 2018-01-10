package namespaces

//go:generate goderive .

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mattn/go-zglob/fastwalk"
)

var hiddens = []string{
	"/lib/modules", "/lib/apparmor", "/lib/firmware", "/lib/modprobe.d", "/lib/recovery-mode", "/lib/udev", "/lib/brltty", "/lib/cgmanager",
	"/lib/crda", "/lib/cryptsetup", "/lib/hdparm", "/lib/ifupdown", "/lib/init", "/lib/linux-sound-base", "/lib/ufw", "/lib/xtables",
	"/lib/security",
}

func myWalk(root string) []string {
	var mu sync.Mutex
	sharedLibs := map[string]bool{}

	walk := func(path string, typ os.FileMode) error {
		if typ.IsDir() {
			if strings.Contains(path, "/security/") {
				return filepath.SkipDir
			}
			if deriveAnyHidden(func(hidden string) bool {
				return strings.HasPrefix(path, hidden)
			}, hiddens) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(path, "/security/") {
			return nil
		}
		if deriveAnyHidden(func(hidden string) bool { return strings.HasPrefix(path, hidden) }, hiddens) {
			return nil
		}
		if strings.HasSuffix(path, ".so") || strings.Contains(path, ".so.") {
			mu.Lock()
			sharedLibs[path] = true
			mu.Unlock()
		}
		return nil
	}
	_ = fastwalk.FastWalk(root, walk)
	return deriveKeys(sharedLibs)

}
