package utils

import (
	"io"
	"sync"
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

// All executes all the provided funcs and returns the first error.
func All(funs ...Func) (err error) {
	errs := make([]error, 0, len(funs))
	for _, f := range funs {
		errs = append(errs, f())
	}
	return AnyErr(errs...)
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

// Parallel executes the provided funcs in parallel and returns one of the returned errors if any.
func Parallel(funs ...Func) error {
	var wg sync.WaitGroup
	errs := make([]error, 0, len(funs))
	errChan := make(chan error)
	finished := make(chan struct{})
	for _, fun := range funs {
		wg.Add(1)
		go func(f func() error) {
			errChan <- f()
			wg.Done()
		}(fun)
	}
	go func() {
		for err := range errChan {
			errs = append(errs, err)
		}
		close(finished)
	}()
	wg.Wait()
	close(errChan)
	<-finished
	return AnyErr(errs...)
}
