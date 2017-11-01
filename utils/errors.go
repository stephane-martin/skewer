package utils

import (
	"errors"

	"github.com/oklog/ulid"
)

var ErrDisposed error = errors.New(`queue: disposed`)
var ErrTimeout = errors.New(`queue: poll timed out`)
var ErrEmptyQueue = errors.New(`queue: empty queue`)
var EmptyUid ulid.ULID
