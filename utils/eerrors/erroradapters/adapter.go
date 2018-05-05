package erroradapters

import (
	"errors"
	"os"
	"syscall"
)

var ErrNetClosing = errors.New("use of closed network connection")
var ErrFileClosing = errors.New("use of closed file")
var ErrNoDeadline = errors.New("file type does not support deadline")

func Adapt(err error) (error, bool) {
	switch e := err.(type) {
	case *invalidError, *closedError, *existError, *notExistError, *noDeadlineError, *permissionError, *pathError, *linkError, *syscallError, *errnoError:
		return err, true
	case *os.PathError:
		return &pathError{e}, true
	case *os.LinkError:
		return &linkError{e}, true
	case *os.SyscallError:
		return &syscallError{e}, true
	case syscall.Errno:
		return &errnoError{e}, true
	}
	switch err {
	case os.ErrInvalid:
		return &invalidError{err}, true
	case os.ErrClosed:
		return &closedError{err}, true
	case os.ErrExist:
		return &existError{err}, true
	case os.ErrNotExist:
		return &notExistError{err}, true
	case os.ErrNoDeadline:
		return &noDeadlineError{err}, true
	case os.ErrPermission:
		return &permissionError{err}, true
	}
	s := err.Error()
	if s == ErrFileClosing.Error() {
		return &closedError{err}, true
	}
	if s == ErrNetClosing.Error() {
		return &closedError{err}, true
	}
	return err, false
}

type invalidError struct{ cause error }
type closedError struct{ cause error }
type existError struct{ cause error }
type notExistError struct{ cause error }
type noDeadlineError struct{ cause error }
type permissionError struct{ cause error }

type pathError struct{ cause *os.PathError }
type linkError struct{ cause *os.LinkError }
type syscallError struct{ cause *os.SyscallError }
type errnoError struct{ cause syscall.Errno }

func (e *invalidError) Cause() error    { return e.cause }
func (e *closedError) Cause() error     { return e.cause }
func (e *existError) Cause() error      { return e.cause }
func (e *notExistError) Cause() error   { return e.cause }
func (e *noDeadlineError) Cause() error { return e.cause }
func (e *permissionError) Cause() error { return e.cause }

func (e *pathError) Cause() error    { return e.cause }
func (e *linkError) Cause() error    { return e.cause }
func (e *syscallError) Cause() error { return e.cause }
func (e *errnoError) Cause() error   { return e.cause }

func (e *invalidError) Error() string    { return e.cause.Error() }
func (e *closedError) Error() string     { return e.cause.Error() }
func (e *existError) Error() string      { return e.cause.Error() }
func (e *notExistError) Error() string   { return e.cause.Error() }
func (e *noDeadlineError) Error() string { return e.cause.Error() }
func (e *permissionError) Error() string { return e.cause.Error() }

func (e *pathError) Error() string    { return e.cause.Error() }
func (e *linkError) Error() string    { return e.cause.Error() }
func (e *syscallError) Error() string { return e.cause.Error() }
func (e *errnoError) Error() string   { return e.cause.Error() }

func (e *invalidError) Invalid() bool       { return true }
func (e *existError) Exist() bool           { return true }
func (e *notExistError) NotExist() bool     { return true }
func (e *permissionError) Permission() bool { return true }
func (e *closedError) Closed() bool         { return true }

func (e *pathError) Exist() bool      { return os.IsExist(e.cause) }
func (e *pathError) NotExist() bool   { return os.IsNotExist(e.cause) }
func (e *pathError) Permission() bool { return os.IsPermission(e.cause) }
func (e *pathError) Timeout() bool    { return os.IsTimeout(e.cause) }
func (e *pathError) Path() bool       { return true }

func (e *linkError) Exist() bool      { return os.IsExist(e.cause) }
func (e *linkError) NotExist() bool   { return os.IsNotExist(e.cause) }
func (e *linkError) Permission() bool { return os.IsPermission(e.cause) }
func (e *linkError) Timeout() bool    { return os.IsTimeout(e.cause) }
func (e *linkError) Link() bool       { return true }

func (e *syscallError) Exist() bool      { return os.IsExist(e.cause) }
func (e *syscallError) NotExist() bool   { return os.IsNotExist(e.cause) }
func (e *syscallError) Permission() bool { return os.IsPermission(e.cause) }
func (e *syscallError) Timeout() bool    { return os.IsTimeout(e.cause) }
func (e *syscallError) Syscall() bool    { return true }

func (e *errnoError) Exist() bool      { return os.IsExist(e.cause) }
func (e *errnoError) NotExist() bool   { return os.IsNotExist(e.cause) }
func (e *errnoError) Permission() bool { return os.IsPermission(e.cause) }
func (e *errnoError) Timeout() bool    { return os.IsTimeout(e.cause) }
func (e *errnoError) Errno() bool      { return true }
