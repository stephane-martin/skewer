package eerrors

import (
	"io"
	"os"
	"syscall"

	errors "github.com/segmentio/errors-go"

	// setting up global adapters for all packages of the standard library supported by the errors-go project
	_ "github.com/segmentio/errors-go/stderrors"
	_ "github.com/stephane-martin/skewer/utils/eerrors/erroradapters"
)

func IsTemporary(err error) bool {
	return errors.Is("Temporary", errors.Adapt(err))
}

func IsTimeout(err error) bool {
	return errors.Is("Timeout", errors.Adapt(err))
}

func IsFatal(err error) bool {
	return errors.Is("Fatal", err)
}

func HasErrno(err error, errno syscall.Errno) bool {
	err = RootCause(err)
	if err == nil {
		return false
	}
	if err == errno {
		return true
	}
	for _, cause := range errors.Causes(err) {
		if HasErrno(cause, errno) {
			return true
		}
	}
	return false
}

func HasBrokenPipe(err error) bool {
	return HasErrno(err, syscall.EPIPE)
}

func HasConnRefused(err error) bool {
	return HasErrno(err, syscall.ECONNREFUSED)
}

func HasFileClosed(err error) bool {
	if err == nil {
		return false
	}

	err = RootCause(err)

	if err == io.EOF || err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF || err == os.ErrClosed {
		return true
	}

	if has, ok := is("Closed", err); has && ok {
		return true
	}

	for _, cause := range errors.Causes(err) {
		if HasFileClosed(cause) {
			return true
		}
	}
	return false
}

func HasEOF(err error) bool {
	err = RootCause(err)
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	for _, cause := range errors.Causes(err) {
		if HasEOF(cause) {
			return true
		}
	}
	return false
}
