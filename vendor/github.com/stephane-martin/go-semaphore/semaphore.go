package semaphore

/*
#include <fcntl.h>
#include <sys/stat.h>
#include <semaphore.h>
#include <stdlib.h>

sem_t *my_sem_open(const char *name, int oflag, mode_t mode, unsigned int value) {
	return sem_open(name, oflag, mode, value);
}

sem_t *my_sem_failed() {
	return SEM_FAILED;
}
*/
import "C"
import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

var semFAILED = C.my_sem_failed()

// PSemaphore is a semaphore handle.
type PSemaphore struct {
	cSem *C.sem_t
	name string
}

// New opens an existing POSIX semaphore by name, or creates a new POSIX semaphore if it does not exist.
//
// When creating, value is the initial value of the semaphore.
func New(name string, value uint) (s *PSemaphore, err error) {
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	semptr, err := C.my_sem_open(cName, C.O_CREAT, C.mode_t(0600), C.uint(value))
	if semptr == semFAILED {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("sem_open returned SEM_FAILED")
	}
	goSem := &PSemaphore{}
	goSem.cSem = semptr
	goSem.name = name
	return goSem, nil
}

// Destroy destroys, eg. removes from the name space, an existing semaphore.
func Destroy(name string) (err error) {
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	var ret C.int
	ret, err = C.sem_unlink(cName)
	if ret != 0 {
		return err
	}
	return nil
}

// Lock locks a semaphore, eg. decrease the value by 1.
//
// Lock blocks until it is possible to lock it.
func (s *PSemaphore) Lock() (err error) {
	if s == nil || s.cSem == nil {
		return errors.New("Not initialized")
	}
	var ret C.int
	ret, err = C.sem_wait(s.cSem)
	if ret != 0 {
		return err
	}
	return nil
}

// TryLock tries to lock a semaphore.
//
// If it is not possible to lock the semaphore, Trylock returns immediatly with an error.
func (s *PSemaphore) TryLock() (err error) {
	if s == nil || s.cSem == nil {
		return errors.New("Not initialized")
	}
	var ret C.int
	ret, err = C.sem_trywait(s.cSem)
	if ret != 0 {
		return err
	}
	return nil
}

// Unlock unlocks a semaphore.
func (s *PSemaphore) Unlock() (err error) {
	if s == nil || s.cSem == nil {
		return errors.New("Not initialized")
	}
	var ret C.int
	ret, err = C.sem_post(s.cSem)
	if ret != 0 {
		return err
	}
	return nil
}

// Close closes an opened semaphore.
//
// The semaphore is closed but not removed from the namespace.
func (s *PSemaphore) Close() (err error) {
	if s == nil || s.cSem == nil {
		return errors.New("Not initialized")
	}
	var ret C.int
	ret, err = C.sem_close(s.cSem)
	if ret != 0 {
		return err
	}
	return nil
}

// Destroy destroys, eg. removes from the name space, an existing semaphore.
func (s *PSemaphore) Destroy() (err error) {
	return Destroy(s.name)
}
