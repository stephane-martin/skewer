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
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	mmap "github.com/edsrzf/mmap-go"
)

/*
int shm_open(const char *path, int flags, mode_t mode);
int shm_unlink(const char *path);
int shm_mkstemp(char *template);
*/

type SharedMem struct {
	mem  mmap.MMap
	f    *os.File
	name string
}

func Create(regionName string, size int) (s *SharedMem, err error) {
	if size <= 0 {
		return nil, fmt.Errorf("Size must be strictly positive")
	}
	if !strings.HasPrefix(regionName, "/") {
		regionName = "/" + regionName
	}
	s = &SharedMem{name: regionName}
	s.f, err = open(regionName, int(C.O_RDWR|C.O_CREAT|C.O_EXCL), 0600)
	if err != nil {
		return nil, err
	}
	err = syscall.Ftruncate(int(s.f.Fd()), int64(size))
	if err != nil {
		return nil, err
	}
	s.mem, err = mmap.MapRegion(s.f, size, mmap.RDWR, 0, 0)
	if err != nil {
		s.f.Close()
		Delete(regionName)
		return nil, err
	}
	return s, nil
}

func Open(regionName string) (s *SharedMem, err error) {
	if !strings.HasPrefix(regionName, "/") {
		regionName = "/" + regionName
	}
	s = &SharedMem{name: regionName}
	s.f, err = open(regionName, int(C.O_RDWR), 0600)
	if err != nil {
		return nil, err
	}
	s.mem, err = mmap.MapRegion(s.f, -1, mmap.RDWR, 0, 0)
	if err != nil {
		s.f.Close()
		return nil, err
	}
	return s, nil
}

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

func (s *SharedMem) Close() (err error) {
	err = s.mem.Unmap()
	if err != nil {
		return err
	}
	return s.f.Close()
}

func (s *SharedMem) Delete() error {
	return Delete(s.name)
}

func (s *SharedMem) Pointer() unsafe.Pointer {
	return unsafe.Pointer(&((s.mem)[0]))
}

func (s *SharedMem) Len() int {
	return len(s.mem)
}

func Delete(regionName string) error {
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
