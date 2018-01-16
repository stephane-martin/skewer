package tail

import (
	"fmt"
	"sync"

	"github.com/oklog/ulid"
)

// FileError is an error that was generated when dealing with some given filename
type FileError struct {
	Filename string
	Err      error
}

type FileErrorID struct {
	FileError
	Uid ulid.ULID
}

func (err FileError) Error() string {
	return fmt.Sprintf("Error processing '%s': %s", err.Filename, err.Err.Error())
}

func prefixErrors(input chan error, output chan error, filename string, wg *sync.WaitGroup) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		for err := range input {
			if output != nil {
				output <- FileError{Filename: filename, Err: err}
			}
		}
	}()
}

func prefixErrorsID(input chan error, output chan error, filename string, uids []ulid.ULID, wg *sync.WaitGroup) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		for err := range input {
			if output != nil {
				for _, uid := range uids {
					output <- FileErrorID{
						FileError: FileError{
							Filename: filename,
							Err:      err,
						},
						Uid: uid,
					}
				}
			}
		}
	}()
}

func prefixLine(input chan []byte, output chan FileLine, filename string, wg *sync.WaitGroup) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		for line := range input {
			if output != nil {
				output <- FileLine{Filename: filename, Line: line}
			}
		}
	}()
}

func prefixLineID(input chan []byte, output chan FileLineID, filename string, uids []ulid.ULID, wg *sync.WaitGroup) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		for line := range input {
			if output != nil {
				for _, uid := range uids {
					output <- FileLineID{
						FileLine: FileLine{
							Filename: filename,
							Line:     line,
						},
						Uid: uid,
					}
				}
			}
		}
	}()
}
