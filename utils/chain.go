package utils

import (
	"io"

	"github.com/golang/sync/errgroup"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

type Func func() error

// Chain executes the provided funcs until an error is returned.
func Chain(funs ...Func) (err error) {
	for _, f := range funs {
		err = f()
		if err != nil {
			return err
		}
	}
	return nil
}

// All executes all the provided funcs and returns the errors.
func All(funs ...Func) (err eerrors.ErrorSlice) {
	c := eerrors.ChainErrors()
	for _, f := range funs {
		c.Append(f())
	}
	return c.Sum()
}

// ChainWrites writes the provided buffers until an error is returned.
func ChainWrites(dest io.Writer, buffers ...[]byte) (err error) {
	for _, b := range buffers {
		if len(b) == 0 {
			continue
		}
		_, err = dest.Write(b)
		if err != nil {
			return err
		}
	}
	return nil
}

// AnyErr returns the first error from the provided list.
func AnyErr(errs ...error) (err error) {
	for _, err = range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// Parallel executes the provided funcs in parallel and returns the errors
func Parallel(funs ...Func) eerrors.ErrorSlice {
	var g errgroup.Group
	c := eerrors.ChainErrors()
	for _, fun := range funs {
		f := fun
		g.Go(
			func() error {
				err := f()
				c.Append(err)
				return err
			},
		)
	}
	g.Wait()
	return c.Sum()
}
