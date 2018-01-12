// +build !linux

package shm

/*
#include <sys/mman.h>
#include <stdlib.h>
#include <fcntl.h>

int my_shm_open(const char *path, int flags, mode_t mode) {
	return shm_open(path, flags, mode);
}
*/
import "C"
import (
	"os"
	"strings"
	"unsafe"
)

/*
int shm_open(const char *path, int flags, mode_t mode);
int shm_unlink(const char *path);
int shm_mkstemp(char *template);
*/

func open(regionName string, flags int, perm os.FileMode) (f *os.File, err error) {
	cname := C.CString(regionName)
	defer C.free(unsafe.Pointer(cname))
	var fd C.int
	fd, err = C.my_shm_open(cname, C.int(flags), C.mode_t(perm))
	if fd < 0 {
		return nil, err
	}
	f = os.NewFile(uintptr(fd), regionName)
	return f, nil
}

func del(regionName string) error {
	if !strings.HasPrefix(regionName, "/") {
		regionName = "/" + regionName
	}
	cname := C.CString(regionName)
	defer C.free(unsafe.Pointer(cname))
	res, err := C.shm_unlink(cname)
	if res != 0 {
		return err
	}
	return nil
}
