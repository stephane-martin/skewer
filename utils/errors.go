package utils

import (
	"errors"
	"io"
	"net"
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

func IsTemporary(err error) bool {
	if nerr, ok := err.(net.Error); ok {
		return nerr.Temporary()
	}
	return false
}

func IsTimeout(err error) bool {
	if nerr, ok := err.(net.Error); ok {
		return nerr.Timeout()
	}
	return false
}

func IsBrokenPipe(err error) bool {
	if err == nil {
		return false
	}
	if perr, ok := err.(*os.PathError); ok {
		if serr, ok := (perr.Err).(*os.SyscallError); ok {
			return serr.Err == syscall.EPIPE
		}
	}
	if operr, ok := err.(*net.OpError); ok {
		if serr, ok := (operr.Err).(*os.SyscallError); ok {
			return serr.Err == syscall.EPIPE
		}
	}
	return false
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
