package queue

import "errors"

var ErrDisposed error = errors.New(`queue: disposed`)
