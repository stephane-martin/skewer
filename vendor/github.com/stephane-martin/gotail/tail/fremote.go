package tail

import (
	"os"
	"syscall"
)

func fremote(f *os.File) (b bool, err error) {
	stats := syscall.Statfs_t{}
	err = syscall.Fstatfs(int(f.Fd()), &stats)
	if err != nil {
		return
	}
	/*
		fsname := make([]byte, len(stats.Fstypename))
		for i, c := range stats.Fstypename {
			fsname[i] = byte(c)
		}
		fmt.Fprintln(os.Stderr, string(fsname))
	*/
	b = !isLocalFS(int64(stats.Type))
	return
}
