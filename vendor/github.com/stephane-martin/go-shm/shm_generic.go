package shm

/*
#include <stdlib.h>
#include <fcntl.h>
*/
import "C"
import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

type sharedMem struct {
	mem  MMap
	f    *os.File
	name string
}

type SharedMemory interface {
	Close() error
	Len() int
	Pointer() unsafe.Pointer
	Delete() error
}

func Create(regionName string, size int) (sh SharedMemory, err error) {
	if size <= 0 {
		return nil, fmt.Errorf("Size must be strictly positive")
	}
	if !strings.HasPrefix(regionName, "/") {
		regionName = "/" + regionName
	}
	s := sharedMem{name: regionName}
	s.f, err = open(regionName, int(C.O_RDWR|C.O_CREAT|C.O_EXCL), 0600)
	if err != nil {
		return nil, err
	}
	err = syscall.Ftruncate(int(s.f.Fd()), int64(size))
	if err != nil {
		return nil, err
	}
	s.mem, err = MapRegion(s.f, size, RDWR, 0, 0)
	if err != nil {
		_ = s.f.Close()
		_ = del(regionName)
		return nil, err
	}
	return &s, nil
}

func Open(regionName string) (sh SharedMemory, err error) {
	if !strings.HasPrefix(regionName, "/") {
		regionName = "/" + regionName
	}
	s := sharedMem{name: regionName}
	s.f, err = open(regionName, int(C.O_RDWR), 0600)
	if err != nil {
		return nil, err
	}
	s.mem, err = MapRegion(s.f, -1, RDWR, 0, 0)
	if err != nil {
		_ = s.f.Close()
		return nil, err
	}
	return &s, nil
}

func (s *sharedMem) Close() (err error) {
	err = s.mem.Unmap()
	if err != nil {
		return err
	}
	return s.f.Close()
}

func (s *sharedMem) Delete() error {
	return del(s.name)
}

func (s *sharedMem) Pointer() unsafe.Pointer {
	return unsafe.Pointer(&((s.mem)[0]))
}

func (s *sharedMem) Len() int {
	return len(s.mem)
}
