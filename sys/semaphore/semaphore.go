package semaphore

/*
#include <fcntl.h>
#include <sys/stat.h>
#include <semaphore.h>
#include <stdlib.h>

sem_t *my_sem_open(const char *name, int oflag, mode_t mode, unsigned int value) {
	return sem_open(name, oflag, mode, value);
}

sem_t * my_sem_failed() {
	return SEM_FAILED;
}
*/
import "C"
import (
	"fmt"
	"strings"
	"unsafe"
)

type Semaphore struct {
	sem  *C.sem_t
	name string
}

func New(name string) (s *Semaphore, err error) {
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	s = &Semaphore{name: name}
	cName := C.CString(s.name)
	defer C.free(unsafe.Pointer(cName))
	s.sem, err = C.my_sem_open(cName, C.O_CREAT, C.mode_t(0600), C.uint(1))
	if s.sem == C.my_sem_failed() {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("sem_open returned SEM_FAILED")
	}
	return s, nil
}

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

func (s *Semaphore) Lock() (err error) {
	var ret C.int
	ret, err = C.sem_wait(s.sem)
	if ret != 0 {
		return err
	}
	return nil
}

func (s *Semaphore) TryLock() (err error) {
	var ret C.int
	ret, err = C.sem_trywait(s.sem)
	if ret != 0 {
		return err
	}
	return nil
}

func (s *Semaphore) Unlock() (err error) {
	var ret C.int
	ret, err = C.sem_post(s.sem)
	if ret != 0 {
		return err
	}
	return nil
}

func (s *Semaphore) Close() (err error) {
	var ret C.int
	ret, err = C.sem_close(s.sem)
	if ret != 0 {
		return err
	}
	return nil
}
