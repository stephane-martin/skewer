package tail

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type TailFileOpts struct {
	filename string
	nbLines  int
	results  chan string
	errors   chan error
	wg       *sync.WaitGroup
	period   time.Duration
}

type FileLine struct {
	Filename string
	Line     string
}

type TailFilesOpts struct {
	filenames map[string]bool
	nbLines   int
	results   chan FileLine
	errors    chan error
	period    time.Duration
}

type TailFilesOpt func(opts *TailFilesOpts)

func (opts *TailFilesOpts) allFiles() (r []string) {
	r = make([]string, 0, len(opts.filenames))
	for fname := range opts.filenames {
		r = append(r, fname)
	}
	sort.Strings(r)
	return r
}

func MSleepPeriod(period time.Duration) TailFilesOpt {
	return func(opts *TailFilesOpts) {
		if period > 0 {
			opts.period = period
		}
	}
}

func MFilename(filename string) TailFilesOpt {
	return func(opts *TailFilesOpts) {
		filename = strings.TrimSpace(filename)
		if len(filename) > 0 {
			if opts.filenames == nil {
				opts.filenames = map[string]bool{}
			}
			opts.filenames[filename] = true
		}
	}
}

func MFilenames(filenames []string) TailFilesOpt {
	return func(opts *TailFilesOpts) {
		for _, fname := range filenames {
			if len(fname) > 0 {
				if opts.filenames == nil {
					opts.filenames = map[string]bool{}
				}
				opts.filenames[fname] = true
			}
		}
	}
}

func MNLines(nbLines int) TailFilesOpt {
	return func(opts *TailFilesOpts) {
		if nbLines >= 0 {
			opts.nbLines = nbLines
		}
	}
}

func MLinesChan(results chan FileLine) TailFilesOpt {
	return func(opts *TailFilesOpts) {
		if results != nil {
			opts.results = results
		}
	}
}

func MErrorChan(errors chan error) TailFileOpt {
	return func(opts *TailFileOpts) {
		if errors != nil {
			opts.errors = errors
		}
	}
}

type TailFileOpt func(opts *TailFileOpts)

func waitgroup(wg *sync.WaitGroup) TailFileOpt {
	return func(opts *TailFileOpts) {
		opts.wg = wg
	}
}

func SleepPeriod(period time.Duration) TailFileOpt {
	return func(opts *TailFileOpts) {
		if period > 0 {
			opts.period = period
		}
	}
}

func Filename(filename string) TailFileOpt {
	return func(opts *TailFileOpts) {
		filename = strings.TrimSpace(filename)
		if len(filename) > 0 {
			opts.filename = filename
		}
	}
}

func NLines(nbLines int) TailFileOpt {
	return func(opts *TailFileOpts) {
		if nbLines >= 0 {
			opts.nbLines = nbLines
		}
	}
}

func LinesChan(results chan string) TailFileOpt {
	return func(opts *TailFileOpts) {
		if results != nil {
			opts.results = results
		}
	}
}

func ErrorChan(errors chan error) TailFileOpt {
	return func(opts *TailFileOpts) {
		if errors != nil {
			opts.errors = errors
		}
	}
}
