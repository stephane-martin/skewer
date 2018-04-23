package eerrors

import errors "github.com/segmentio/errors-go"

// ErrQDisposed is returned when a queue is accessed after being disposed.
var ErrQDisposed = errors.WithTypes(errors.New("queue: disposed"), "Disposed")

// ErrQTimeout is returned by queues after the provided timeout is expired.
var ErrQTimeout = errors.WithTypes(errors.New("queue: poll timed out"), "Timeout")

// ErrQEmpty is returned when an non-applicable queue operation was called
var ErrQEmpty = errors.New("queue: empty queue")
