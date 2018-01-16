package tail

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mattn/go-zglob/fastwalk"
	"github.com/oklog/ulid"
)

// Tailor can be used to monitor whole directories and report new lines in files
// that live inside the directories.
type Tailor struct {
	results       chan FileLineID
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
func NewTailor(results chan FileLineID, errors chan error) (t *Tailor, err error) {
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
						t.logerror(
							t.watcher.Add(absName),
						)
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
			uids := t.directories.Filter(absName)
			if len(uids) > 0 {
				// we should tail that new file
				t.addFile(absName, uids, true, &t.mu)
				//fmt.Fprintln(os.Stderr, "new file to tail:", absName)
			}
			uids = t.rdirectories.RFilter(absName)
			if len(uids) > 0 {
				// we should tail that new file
				t.addFile(absName, uids, true, &t.mu)
				//fmt.Fprintln(os.Stderr, "new file to tail:", absName)
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
func (t *Tailor) AddDirectory(dirname string, filter FilterFunc) (uid ulid.ULID, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !isDir(dirname) {
		return uid, fmt.Errorf("Not a directory: '%s'", dirname)
	}

	absName := t.abs(dirname)
	filenames, err := t.ls(absName)
	if err != nil {
		return uid, err
	}
	err = t.watcher.Add(absName) // watch the directory for newly created files
	if err != nil {
		return uid, err
	}
	uid, err = ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		return uid, err
	}
	t.directories.Add(absName, filter, uid)
	//fmt.Fprintln(os.Stderr, "Watching dir:", absName)

	// tail the existing files in the directory
	for _, fname := range filenames {
		//fmt.Fprintln(os.Stderr, "examine file:", fname, "=", relname)
		uids := t.directories.Filter(fname)
		if len(uids) > 0 {
			t.addFile(fname, uids, false, nil) // NB: we already have the t.mu lock
		}
	}
	return uid, nil
}

// AddRecursiveDirectory tells the Tailor to watch globally and recursively a directory.
// The existing files and the new created files inside dirname will be monitored
// for new content. Sub-directories of dirname are added too.
// The filter is a function to select which files to monitor, based on the
// name of a file (relative name of the file relative to the top-most directory).
func (t *Tailor) AddRecursiveDirectory(dirname string, filter FilterFunc) (uid ulid.ULID, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !isDir(dirname) {
		return uid, fmt.Errorf("Not a directory: '%s'", dirname)
	}

	absName := t.abs(dirname)
	files, dirs, err := t.lsrecurse(absName)
	if err != nil {
		return uid, err
	}
	uid, err = ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		return uid, err
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
			return uid, err
		}
		watched = append(watched, dir)
	}
	t.rdirectories.Add(absName, filter, uid)

	// tail the existing files
	for _, fname := range files {
		uids := t.rdirectories.RFilter(fname)
		if len(uids) > 0 {
			//fmt.Fprintln(os.Stderr, "init add file", fname)
			t.addFile(fname, uids, false, nil)
		}
	}
	return uid, nil
}

func (t *Tailor) addFile(filename string, uids []ulid.ULID, new bool, mu *sync.Mutex) {
	//fmt.Fprintln(os.Stderr, "addFile", filename)
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	// avoid duplicates
	filename = t.abs(filename)
	if _, ok := t.fspecs.Load(filename); ok {
		//fmt.Fprintln(os.Stderr, "duplicate", filename)
		return
	}
	results := make(chan []byte)
	errors := make(chan error)
	ctx := context.Background()
	prefixLineID(results, t.results, filename, uids, &t.prefixWg)
	prefixErrorsID(errors, t.errors, filename, uids, &t.prefixWg)
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
		t.n.AddFile(fspec)
	}
	t.fspecs.Store(filename, fspec)
	return
}

// FilterFunc is the type of filter functions.
type FilterFunc func(relname string) bool

/*
type dirFilter struct {
	directory string
	filter    FilterFunc
}
*/

type filterSpec struct {
	f   FilterFunc
	uid ulid.ULID
}

type dirSet struct {
	m sync.Map
	//map[string=dirname]([]FilterSpec)
}

func newDirSet() *dirSet {
	return &dirSet{}
}

func (s *dirSet) Add(dirname string, filter FilterFunc, uid ulid.ULID) {
	filtersI, _ := s.m.Load(dirname)
	if filtersI == nil {
		s.m.Store(dirname, []filterSpec{
			filterSpec{
				f: filter, uid: uid,
			},
		})
		return
	}
	filterspecs := filtersI.([]filterSpec)
	if len(filterspecs) == 0 {
		s.m.Store(dirname, []filterSpec{
			filterSpec{
				f: filter, uid: uid,
			},
		})
		return
	}
	s.m.Store(dirname, append(filterspecs, filterSpec{f: filter, uid: uid}))
	return
}

func (s *dirSet) Filter(filename string) (uids []ulid.ULID) {
	uids = make([]ulid.ULID, 0)
	dirname := filepath.Dir(filename)
	fs, _ := s.m.Load(dirname)
	if fs == nil {
		return uids
	}
	relname := filepath.Base(filename)
	for _, spec := range fs.([]filterSpec) {
		if spec.f(relname) {
			uids = append(uids, spec.uid)
		}
	}
	return uids
}

func (s *dirSet) RFilter(filename string) (uids []ulid.ULID) {
	uids = make([]ulid.ULID, 0)
	s.m.Range(func(k interface{}, v interface{}) bool {
		dname := k.(string)
		if strings.HasPrefix(filename, dname) {
			if relname, err := filepath.Rel(dname, filename); err == nil {
				for _, spec := range v.([]filterSpec) {
					if spec.f(relname) {
						uids = append(uids, spec.uid)
					}
				}
			}
		}
		return true
	})
	return uids
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
