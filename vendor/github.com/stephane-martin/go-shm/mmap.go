// +build darwin dragonfly freebsd linux openbsd solaris netbsd

package shm

import (
	"errors"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

const (
	// RDONLY maps the memory read-only.
	// Attempts to write to the MMap object will result in undefined behavior.
	RDONLY = 0
	// RDWR maps the memory as read-write. Writes to the MMap object will update the
	// underlying file.
	RDWR = 1 << iota
	// COPY maps the memory as copy-on-write. Writes to the MMap object will affect
	// memory, but the underlying file will remain unchanged.
	COPY
	// If EXEC is set, the mapped memory is marked as executable.
	EXEC
)

const (
	// If the ANON flag is set, the mapped memory will not be backed by a file.
	ANON = 1 << iota
)

const _SYS_MSYNC = syscall.SYS_MSYNC
const _MS_SYNC = syscall.MS_SYNC

func mmap(len int, inprot, inflags, fd uintptr, off int64) ([]byte, error) {
	flags := syscall.MAP_SHARED
	prot := syscall.PROT_READ
	switch {
	case inprot&COPY != 0:
		prot |= syscall.PROT_WRITE
		flags = syscall.MAP_PRIVATE
	case inprot&RDWR != 0:
		prot |= syscall.PROT_WRITE
	}
	if inprot&EXEC != 0 {
		prot |= syscall.PROT_EXEC
	}
	if inflags&ANON != 0 {
		flags |= syscall.MAP_ANON
	}

	b, err := syscall.Mmap(int(fd), off, len, prot, flags)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func flush(addr, len uintptr) error {
	_, _, errno := syscall.Syscall(_SYS_MSYNC, addr, len, _MS_SYNC)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}

func lock(addr, len uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_MLOCK, addr, len, 0)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}

func unlock(addr, len uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_MUNLOCK, addr, len, 0)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}

func unmap(addr, len uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_MUNMAP, addr, len, 0)
	if errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}

type MMap []byte

func MapRegion(f *os.File, length int, prot, flags int, offset int64) (MMap, error) {
	if offset%int64(os.Getpagesize()) != 0 {
		return nil, errors.New("offset parameter must be a multiple of the system's page size")
	}

	var fd uintptr
	if flags&ANON == 0 {
		fd = uintptr(f.Fd())
		if length < 0 {
			fi, err := f.Stat()
			if err != nil {
				return nil, err
			}
			length = int(fi.Size())
		}
	} else {
		if length <= 0 {
			return nil, errors.New("anonymous mapping requires non-zero length")
		}
		fd = ^uintptr(0)
	}
	return mmap(length, uintptr(prot), uintptr(flags), fd, offset)
}

func (m *MMap) header() *reflect.SliceHeader {
	return (*reflect.SliceHeader)(unsafe.Pointer(m))
}

// Lock keeps the mapped region in physical memory, ensuring that it will not be
// swapped out.
func (m MMap) Lock() error {
	dh := m.header()
	return lock(dh.Data, uintptr(dh.Len))
}

// Unlock reverses the effect of Lock, allowing the mapped region to potentially
// be swapped out.
// If m is already unlocked, aan error will result.
func (m MMap) Unlock() error {
	dh := m.header()
	return unlock(dh.Data, uintptr(dh.Len))
}

// Flush synchronizes the mapping's contents to the file's contents on disk.
func (m MMap) Flush() error {
	dh := m.header()
	return flush(dh.Data, uintptr(dh.Len))
}

// Unmap deletes the memory mapped region, flushes any remaining changes, and sets
// m to nil.
// Trying to read or write any remaining references to m after Unmap is called will
// result in undefined behavior.
// Unmap should only be called on the slice value that was originally returned from
// a call to Map. Calling Unmap on a derived slice may cause errors.
func (m *MMap) Unmap() error {
	dh := m.header()
	err := unmap(dh.Data, uintptr(dh.Len))
	*m = nil
	return err
}
