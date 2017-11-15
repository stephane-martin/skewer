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

func ChainWrites(dest io.Writer, buffers ...[]byte) (err error) {
	for _, b := range buffers {
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
	for _, f := range funs {
		wg.Add(1)
		go func() {
			errChan <- f()
			wg.Done()
		}()
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
