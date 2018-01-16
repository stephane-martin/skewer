// Package tail provides tools to tail files in go. It also
// provides tools to watch a file for new lines, or to watch
// whole directories for new content.
package tail

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
)

type tailFunc func(nbLines int, w io.Writer) (int64, error)
type dummyFunc func() error

func selectTailFunc(file *os.File, infos os.FileInfo) (dotail tailFunc) {
	var startPos int64
	var endPos int64
	var err error

	if !isRegular(infos.Mode()) {
		dotail = func(nbLines int, w io.Writer) (int64, error) {
			return pipeLines(file, nbLines, w)
		}
	} else {
		startPos, err = file.Seek(0, 1)
		if err != nil {
			dotail = func(nbLines int, w io.Writer) (int64, error) {
				return pipeLines(file, nbLines, w)
			}
		} else {
			endPos, err = file.Seek(0, 2)
			if err != nil {
				file.Seek(startPos, 0)
				dotail = func(nbLines int, w io.Writer) (int64, error) {
					return pipeLines(file, nbLines, w)
				}
			} else {
				dotail = func(nbLines int, w io.Writer) (int64, error) {
					return fileLines(file, nbLines, startPos, endPos, w)
				}
			}
		}
	}
	return dotail
}

// TailFiles returns the N last lines of the given files to a FileLine channel.
// TailFiles returns immediatly. The context can be used to stop the tailing.
// The given channels will be closed at the end of the tailing or after the context has been canceled.
// If given, the FileLine channel must be consumed by the client.
// If given, the error channel must be consumed by the client.
func TailFiles(ctx context.Context, opts ...TailFilesOpt) {
	env := tailFilesOpts{
		nbLines: 10,
	}

	for _, opt := range opts {
		opt(&env)
	}

	filenames := env.allFiles()
	if len(filenames) == 0 {
		if env.results != nil {
			close(env.results)
		}
		if env.errors != nil {
			close(env.errors)
		}
		return
	}

	var wg sync.WaitGroup
	for _, filename := range filenames {
		fname := filename
		errChan := make(chan error)
		resultsChan := make(chan []byte)
		prefixErrors(errChan, env.errors, fname, &wg)
		prefixLine(resultsChan, env.results, fname, &wg)
		err := TailFile(
			ctx,
			Filename(fname),
			NLines(env.nbLines),
			ErrorChan(errChan),
			LinesChan(resultsChan),
			waitgroup(&wg),
		)
		if err != nil && env.errors != nil {
			env.errors <- FileError{Filename: fname, Err: err}
		}
	}
	go func() {
		wg.Wait()
		if env.errors != nil {
			close(env.errors)
		}
		if env.results != nil {
			close(env.results)
		}
	}()
}

// TailFile returns the N last lines of the given file to a string channel.
// TailFile returns immediatly. The context can be used to stop the tailing.
// The given channels will be closed at the end of the tailing or after the context has been canceled.
// If given, the string channel must be consumed by the client.
// If given, the error channel must be consumed by the client.
func TailFile(ctx context.Context, opts ...TailFileOpt) (err error) {
	env := tailFileOpts{
		nbLines: 10,
	}

	for _, opt := range opts {
		opt(&env)
	}

	closeResults := func() {
		if env.results != nil {
			close(env.results)
		}
	}

	closeErrors := func() {
		if env.errors != nil {
			close(env.errors)
		}
	}

	if len(env.filename) == 0 {
		closeResults()
		closeErrors()
		return nil
	}

	var file *os.File
	var infos os.FileInfo
	var closeFile dummyFunc

	if env.filename == "-" {
		file = os.Stdin
		closeFile = func() error {
			return nil
		}
	} else {
		file, err = os.Open(env.filename)
		if err != nil {
			closeResults()
			closeErrors()
			return err
		}
		closeFile = func() error {
			return file.Close()
		}
	}

	infos, err = file.Stat()
	if err != nil {
		closeFile()
		closeResults()
		closeErrors()
		return err
	}

	if !isTailable(infos.Mode()) {
		closeFile()
		closeResults()
		closeErrors()
		return fmt.Errorf("The file is not tailable")
	}

	dotail := selectTailFunc(file, infos)
	w := makeWriter(ctx, env.results)

	if env.wg != nil {
		env.wg.Add(1)
	}
	go func() {
		if env.wg != nil {
			defer env.wg.Done()
		}
		defer func() {
			closeFile() // makes dotail abort if needed
			closeErrors()
			w.Close() // will close env.results
		}()
		var err error
		tailFinished := make(chan struct{})
		go func() {
			_, err = dotail(env.nbLines, w)
			close(tailFinished)
		}()

		select {
		case <-tailFinished:
		case <-ctx.Done():
			return
		}
		if err != nil && env.errors != nil {
			env.errors <- err
		}

	}()
	return nil
}
