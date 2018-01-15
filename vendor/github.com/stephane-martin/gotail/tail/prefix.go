package tail

import (
	"fmt"
	"sync"
)

// FileError is an error that was generated when dealing with some given filename
type FileError struct {
	Filename string
	Err      error
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

func prefixLine(input chan string, output chan FileLine, filename string, wg *sync.WaitGroup) {
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
