package eerrors

import (
	"net"
	"net/url"
	"os"
	"syscall"

	errors "github.com/segmentio/errors-go"
)

func RootCause(err error) error {
	for {
		switch e := err.(type) {
		case *os.PathError:
			err = e.Err
			continue
		case *os.LinkError:
			err = e.Err
			continue
		case *os.SyscallError:
			err = e.Err
			continue
		case *url.Error:
			err = e.Err
			continue
		case *net.OpError:
			err = e.Err
			continue
		}
		if e, ok := err.(cause); ok {
			if cause := e.Cause(); cause != nil {
				err = cause
				continue
			}
		}
		return err
	}
}

func Cause(err error) error {
	return errors.Cause(err)
}

func Causes(err error) []error {
	return errors.Causes(err)
}

func Errno(err error) error {
	err = RootCause(err)
	if err == nil {
		return nil
	}
	if e, ok := err.(syscall.Errno); ok {
		return e
	}
	return nil
}

func WithTypes(err error, types ...string) error {
	return errors.WithTypes(err, types...)
}

func WithMessage(err error, msg string) error {
	return errors.WithMessage(err, msg)
}

func WithTags(err error, tags ...string) error {
	if len(tags) < 2 {
		return err
	}
	keyFlag := true
	key := ""
	value := ""
	etags := make([]errors.Tag, 0, len(tags)/2)
	for _, tag := range tags {
		if keyFlag {
			key = tag
		} else {
			value = tag
			etags = append(etags, errors.T(key, value))
		}
		keyFlag = !keyFlag
	}
	if len(tags) == 0 {
		return err
	}
	return errors.WithTags(err, etags...)
}

func Is(typ string, err error) bool {
	return errors.Is(typ, err)
}

func Err(v interface{}) error {
	return errors.Err(v)
}

func New(msg string) error {
	return errors.New(msg)
}

func Errorf(msg string, args ...interface{}) error {
	return errors.Errorf(msg, args...)
}

func Wrap(err error, msg string) error {
	return errors.Wrap(err, msg)
}

func Wrapf(err error, msg string, args ...interface{}) error {
	return errors.Wrapf(err, msg, args...)
}
