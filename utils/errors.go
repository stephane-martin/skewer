package utils

import (
	"errors"

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
