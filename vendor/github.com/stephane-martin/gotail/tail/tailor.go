package tail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mattn/go-zglob/fastwalk"
)

// Tailor can be used to monitor whole directories and report new lines in files
// that live inside the directories.
type Tailor struct {
	results       chan FileLine
	errors        chan error
	n             *notifier
	mu            sync.Mutex
	watcher       *fsnotify.Watcher
	directories   *dirSet
	rdirectories  *dirSet
	fspecs        sync.Map
	prefixWg      sync.WaitGroup
	cancelPolling chan struct{}
	pollingWg     sync.WaitGroup
	cwd           string
}

// NewTailor builds a *Tailor.
//
// The detected new lines will be sent in the results channel. Errors that may happen
// in processing will be sent to errors.
//
// Both results and errors channels can be nil. If one of them is not nil,
// it must be consumed by the client.
func NewTailor(results chan FileLine, errors chan error) (t *Tailor, err error) {
	t = &Tailor{
		results:       results,
		errors:        errors,
		directories:   newDirSet(),
		rdirectories:  newDirSet(),
		cancelPolling: make(chan struct{}),
	}
	t.n, err = newNotifier(errors)
	if err != nil {
		return nil, err
	}
	t.cwd, err = os.Getwd()
	if err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	t.watcher = w
	err = t.n.Start()
	if err != nil {
		w.Close()
		return nil, err
	}
	go func() {
		for ev := range w.Events {
			if ev.Op != fsnotify.Create && ev.Op != fsnotify.Write {
				continue
			}
			absName := t.abs(ev.Name)
			//fmt.Fprintln(os.Stderr, "event", ev.Op, "for", ev.Name)

			if isDir(absName) {
				if ev.Op == fsnotify.Create {
					// a new directory was created
					if t.rdirectories.HasSubdir(absName) {
						// it is a subdirectory of a directory we have to monitor recursively
						t.logerror(t.watcher.Add(absName))
						//fmt.Fprintln(os.Stderr, "new watch", absName)
					}
				}
				continue
			}

			if _, ok := t.fspecs.Load(absName); ok {
				// anyway we already have a fspec for that file, nothing to do
				continue
			}

			// a regular file was created/modified
			if t.directories.Filter(absName) {
				// we should tail that new file
				//fmt.Fprintln(os.Stderr, "new file to tail:", absName)
				t.logerror(t.addFile(absName, true, &t.mu))
				continue
			}
			if t.rdirectories.RFilter(absName) {
				// we should tail that new file
				//fmt.Fprintln(os.Stderr, "new file to tail:", absName)
				t.logerror(t.addFile(absName, true, &t.mu))
			}
		}
	}()
	go func() {
		for err := range w.Errors {
			t.logerror(err)
		}
	}()
	return t, nil
}

func (t *Tailor) logerror(err error) {
	if t.errors != nil && err != nil {
		t.errors <- err
	}
}

func (t *Tailor) abs(rel string) string {
	// filepath.Abs make a getcwd call each time... avoid it.
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(t.cwd, rel)
}

// Close stops the Tailor. New content will not be detected anymore. Eventually
// the results and errors channels will be closed.
func (t *Tailor) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.watcher != nil {
		t.watcher.Close()
		t.watcher = nil
	}
	if t.n != nil {
		// stop the notifier
		t.n.Stop()
		t.n = nil
	}
	if t.cancelPolling != nil {
		// stops the polling followers
		close(t.cancelPolling)
		// wait for the polling followers to finish
		t.pollingWg.Wait()
		t.cancelPolling = nil
	}
	t.fspecs.Range(func(k interface{}, v interface{}) bool {
		v.(*fileSpec).close()
		return true
	})
	t.prefixWg.Wait()
	if t.results != nil {
		close(t.results)
		t.results = nil
	}
	if t.errors != nil {
		close(t.errors)
		t.errors = nil
	}
}

// CloseOnContext closes the Tailor when the given context is canceled.
func (t *Tailor) CloseOnContext(ctx context.Context) {
	go func() {
		<-ctx.Done()
		t.Close()
	}()
}

func (t *Tailor) ls(dirname string) (r []string, err error) {
	dirname = t.abs(dirname)
	var mu sync.Mutex
	r = make([]string, 0)
	f := func(path string, typ os.FileMode) error {
		path = t.abs(path)
		if typ.IsDir() {
			if path != dirname {
				return filepath.SkipDir
			}
			return nil
		}
		mu.Lock()
		r = append(r, path)
		mu.Unlock()
		return nil
	}
	err = fastwalk.FastWalk(dirname, f)
	return r, err
}

func (t *Tailor) lsrecurse(dirname string) (files []string, dirs []string, err error) {
	var mu sync.Mutex
	files = make([]string, 0)
	dirs = make([]string, 0)
	f := func(path string, typ os.FileMode) error {
		absName := t.abs(path)
		mu.Lock()
		if typ.IsDir() {
			dirs = append(dirs, absName)
		} else {
			files = append(files, absName)
		}
		mu.Unlock()
		return nil
	}
	err = fastwalk.FastWalk(dirname, f)
	return files, dirs, err
}

// AddDirectory tells the Tailor to watch globally a directory.
// The existing files and the new created files directly under dirname will be monitored
// for new content. Sub-directories are not added.
// The filter is a function to select which files to monitor, based on the
// name of a file (relative name of the file in its parent directory).
func (t *Tailor) AddDirectory(dirname string, filter FilterFunc) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !isDir(dirname) {
		return fmt.Errorf("Not a directory: '%s'", dirname)
	}

	absName := t.abs(dirname)
	filenames, err := t.ls(absName)
	if err != nil {
		return err
	}
	err = t.watcher.Add(absName) // watch the directory for newly created files
	if err != nil {
		return err
	}
	t.directories.Add(absName, filter)
	//fmt.Fprintln(os.Stderr, "Watching dir:", absName)

	// tail the existing files in the directory
	for _, fname := range filenames {
		relname := filepath.Base(fname)
		//fmt.Fprintln(os.Stderr, "examine file:", fname, "=", relname)
		if filter(relname) {
			err = t.addFile(fname, false, nil) // NB: we already have the t.mu lock
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// AddRecursiveDirectory tells the Tailor to watch globally and recursively a directory.
// The existing files and the new created files inside dirname will be monitored
// for new content. Sub-directories of dirname are added too.
// The filter is a function to select which files to monitor, based on the
// name of a file (relative name of the file relative to the top-most directory).
func (t *Tailor) AddRecursiveDirectory(dirname string, filter FilterFunc) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !isDir(dirname) {
		return fmt.Errorf("Not a directory: '%s'", dirname)
	}

	absName := t.abs(dirname)
	files, dirs, err := t.lsrecurse(absName)
	if err != nil {
		return err
	}
	//fmt.Fprintln(os.Stderr, "dirs", dirs)
	//fmt.Fprintln(os.Stderr, "files", files)
	watched := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		err = t.watcher.Add(dir) // watch the directories for newly created files
		if err != nil {
			for _, d := range watched {
				t.watcher.Remove(d)
			}
			return err
		}
		watched = append(watched, dir)
	}
	t.rdirectories.Add(absName, filter)

	// tail the existing files
	for _, fname := range files {
		relname, err := filepath.Rel(absName, fname)
		if err != nil {
			return err
		}
		if filter(relname) {
			//fmt.Fprintln(os.Stderr, "init add file", fname)
			err = t.addFile(fname, false, nil)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// AddFiles tells the Tailor to watch some files.
func (t *Tailor) AddFiles(filenames []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, fname := range filenames {
		t.addFile(fname, false, nil)
	}
	return nil
}

// AddFile tells the Tailor to watch a single file.
func (t *Tailor) AddFile(filename string) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	err = t.addFile(filename, false, nil)
	return err
}

func (t *Tailor) addFile(filename string, new bool, mu *sync.Mutex) (err error) {
	//fmt.Fprintln(os.Stderr, "addFile", filename)
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	// avoid duplicates
	filename = t.abs(filename)
	if _, ok := t.fspecs.Load(filename); ok {
		//fmt.Fprintln(os.Stderr, "duplicate", filename)
		return nil
	}
	results := make(chan string)
	errors := make(chan error)
	ctx := context.Background()
	prefixLine(results, t.results, filename, &t.prefixWg)
	prefixErrors(errors, t.errors, filename, &t.prefixWg)
	fspec := makeFspec(ctx, filename, results, errors)
	var nbLines int
	if new {
		nbLines = -1
	}
	//fmt.Fprintln(os.Stderr, "initTail", filename)
	fspec.initTail(ctx, nil, nbLines)
	if fspec.hasClassicalFollow() {
		followClassical(t.cancelPolling, &t.pollingWg, fspec, time.Second)
	} else {
		err = t.n.AddFile(fspec)
	}
	if err != nil {
		fspec.close()
		return err
	}
	t.fspecs.Store(filename, fspec)
	return nil
}

// FilterFunc is the type of filter functions.
type FilterFunc func(relname string) bool

type dirFilter struct {
	directory string
	filter    FilterFunc
}

type dirSet struct {
	m sync.Map
	//m map[string]([]FilterFunc)
}

func newDirSet() *dirSet {
	return &dirSet{}
}

func (s *dirSet) Add(dirname string, filter FilterFunc) {
	filtersI, _ := s.m.Load(dirname)
	if filtersI == nil {
		s.m.Store(dirname, []FilterFunc{filter})
		return
	}
	filters := filtersI.([]FilterFunc)
	if len(filters) == 0 {
		s.m.Store(dirname, []FilterFunc{filter})
	}
	s.m.Store(dirname, append(filters, filter))
	return
}

func (s *dirSet) Filter(filename string) bool {
	dirname := filepath.Dir(filename)
	fs, _ := s.m.Load(dirname)
	if fs == nil {
		return false
	}
	relname := filepath.Base(filename)
	for _, filter := range fs.([]FilterFunc) {
		if filter(relname) {
			return true
		}
	}
	return false
}

func (s *dirSet) RFilter(filename string) (res bool) {
	s.m.Range(func(k interface{}, v interface{}) bool {
		dname := k.(string)
		if strings.HasPrefix(filename, dname) {
			if relname, err := filepath.Rel(dname, filename); err == nil {
				for _, filter := range v.([]FilterFunc) {
					if filter(relname) {
						res = true
						return false
					}
				}
			}
		}
		return true
	})
	return res
}

func (s *dirSet) HasSubdir(dirname string) (res bool) {
	s.m.Range(func(k interface{}, v interface{}) bool {
		if strings.HasPrefix(dirname, k.(string)) {
			res = true
			return false
		}
		return true
	})
	return res
}
