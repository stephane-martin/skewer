package utils

import (
	"io"
	"sync"
)

func Chain(funs ...func() error) (err error) {
	for _, f := range funs {
		err = f()
		if err != nil {
			return err
		}
	}
	return nil
}

func All(funs ...func() error) (err error) {
	errs := make([]error, 0, len(funs))
	for _, f := range funs {
		errs = append(errs, f())
	}
	return AnyErr(errs...)
}

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

func AnyErr(errs ...error) (err error) {
	for _, err = range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func Parallel(funs ...func() error) error {
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
