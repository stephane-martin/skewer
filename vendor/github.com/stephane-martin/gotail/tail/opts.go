package tail

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type tailFileOpts struct {
	filename string
	nbLines  int
	results  chan string
	errors   chan error
	wg       *sync.WaitGroup
	period   time.Duration
}

// FileLine is used to report a line and a filename.
type FileLine struct {
	Filename string
	Line     string
}

type tailFilesOpts struct {
	filenames map[string]bool
	nbLines   int
	results   chan FileLine
	errors    chan error
	period    time.Duration
}

// TailFilesOpt is the type for the options of TailFiles and FollowFiles
type TailFilesOpt func(opts *tailFilesOpts)

// TailFileOpt is the type for the options of TailFile and FollowFile
type TailFileOpt func(opts *tailFileOpts)

func (opts *tailFilesOpts) allFiles() (r []string) {
	r = make([]string, 0, len(opts.filenames))
	for fname := range opts.filenames {
		r = append(r, fname)
	}
	sort.Strings(r)
	return r
}

// MSleepPeriod is the TailFiles and FollowFiles option to set the pause period between pollings in
// poller mode.
func MSleepPeriod(period time.Duration) TailFilesOpt {
	return func(opts *tailFilesOpts) {
		if period > 0 {
			opts.period = period
		}
	}
}

// MFilename is the TailFiles and FollowFiles option to add a file to the list of files to tail/watch.
func MFilename(filename string) TailFilesOpt {
	return func(opts *tailFilesOpts) {
		filename = strings.TrimSpace(filename)
		if len(filename) > 0 {
			if opts.filenames == nil {
				opts.filenames = map[string]bool{}
			}
			opts.filenames[filename] = true
		}
	}
}

// MFilenames is the TailFiles and FollowFiles option to set the list of files to tail/watch.
func MFilenames(filenames []string) TailFilesOpt {
	return func(opts *tailFilesOpts) {
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

// MNLines is the TailFiles and FollowFiles option to set the number of lines to display from the end of files
func MNLines(nbLines int) TailFilesOpt {
	return func(opts *tailFilesOpts) {
		if nbLines >= 0 {
			opts.nbLines = nbLines
		}
	}
}

// MLinesChan is the TailFiles and FollowFiles option to set the channel that will be used to send resulting lines
func MLinesChan(results chan FileLine) TailFilesOpt {
	return func(opts *tailFilesOpts) {
		if results != nil {
			opts.results = results
		}
	}
}

// MErrorChan is the TailFiles and FollowFiles option to set the channel where to send errors
func MErrorChan(errors chan error) TailFileOpt {
	return func(opts *tailFileOpts) {
		if errors != nil {
			opts.errors = errors
		}
	}
}

func waitgroup(wg *sync.WaitGroup) TailFileOpt {
	return func(opts *tailFileOpts) {
		opts.wg = wg
	}
}

// SleepPeriod is the TailFile and FollowFile option to set the pause period between pollings in
// poller mode.
func SleepPeriod(period time.Duration) TailFileOpt {
	return func(opts *tailFileOpts) {
		if period > 0 {
			opts.period = period
		}
	}
}

// Filename is the TailFile and FollowFile option to set the file to tail/watch
func Filename(filename string) TailFileOpt {
	return func(opts *tailFileOpts) {
		filename = strings.TrimSpace(filename)
		if len(filename) > 0 {
			opts.filename = filename
		}
	}
}

// NLines is the TailFile and FollowFile option to set the number of lines to display from the end of files
func NLines(nbLines int) TailFileOpt {
	return func(opts *tailFileOpts) {
		if nbLines >= 0 {
			opts.nbLines = nbLines
		}
	}
}

// LinesChan is the TailFile and FollowFile option to set the channel that will be used to send resulting lines
func LinesChan(results chan string) TailFileOpt {
	return func(opts *tailFileOpts) {
		if results != nil {
			opts.results = results
		}
	}
}

// ErrorChan is the TailFile and FollowFile option to set the channel where to send errors
func ErrorChan(errors chan error) TailFileOpt {
	return func(opts *tailFileOpts) {
		if errors != nil {
			opts.errors = errors
		}
	}
}
