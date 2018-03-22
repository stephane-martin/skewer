package utils

import (
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"syscall"

	"github.com/oklog/ulid"
)

// ErrDisposed is returned when a queue is accessed after being disposed.
var ErrDisposed = errors.New(`queue: disposed`)

// ErrTimeout is returned by queues after the provided timeout is expired.
var ErrTimeout = errors.New(`queue: poll timed out`)

// ErrEmptyQueue is returned when an non-applicable queue operation was called
var ErrEmptyQueue = errors.New(`queue: empty queue`)

// EmptyUID is a zero ULID
var EmptyUID ulid.ULID

type tempo interface {
	Temporary() bool
}

type tout interface {
	Timeout() bool
}

func IsTemporary(err error) bool {
	if nerr, ok := err.(tempo); ok {
		return nerr.Temporary()
	}
	return false
}

func IsTimeout(err error) bool {
	if nerr, ok := err.(tout); ok {
		return nerr.Timeout()
	}
	return false
}

func IsErrno(err error, no syscall.Errno) bool {
	if err == nil {
		return false
	}
	switch e := err.(type) {
	case *os.SyscallError:
		return e.Err == no
	case *os.PathError:
		return IsErrno(e.Err, no)
	case *os.LinkError:
		return IsErrno(e.Err, no)
	case *net.OpError:
		return IsErrno(e.Err, no)
	case *net.AddrError:
		return false
	case *net.DNSError:
		return false
	case *url.Error:
		return IsErrno(e.Err, no)
	default:
		return false
	}
	return false
}

func IsBrokenPipe(err error) bool {
	return IsErrno(err, syscall.EPIPE)
}

func IsConnRefused(err error) bool {
	return IsErrno(err, syscall.ECONNREFUSED)
}

func IsFileClosed(err error) bool {
	if e, ok := err.(*os.PathError); ok {
		err = e.Err
	}
	if err == io.EOF || err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF || err == os.ErrClosed {
		return true
	}
	return false
}
