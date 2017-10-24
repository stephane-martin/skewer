package utils

import "io"

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
