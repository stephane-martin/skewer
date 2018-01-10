package tail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fatih/set"
	"github.com/fsnotify/fsnotify"
	zglob "github.com/mattn/go-zglob"
	"github.com/mattn/go-zglob/fastwalk"
	"github.com/stephane-martin/skewer/utils"
)

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

			if utils.IsDir(absName) {
				if ev.Op == fsnotify.Create {
					// a new directory was created
					dfs := t.rdirectories.RGet(absName)
					if len(dfs) > 0 {
						// it is a subdirectory of a directory we have to monitor recursively
						err = t.watcher.Add(absName)
						if err != nil && t.errors != nil {
							t.errors <- err
						}
					}
				}
				continue
			}

			if _, ok := t.fspecs.Load(absName); ok {
				// anyway we already have a fspec for that file, nothing to do
				continue
			}

			// a regular file was created/modified
			dirName := filepath.Dir(absName)
			filters := t.directories.Get(dirName)
			if len(filters) > 0 {
				// a file was created directly in one of the directories we have to monitor directly
				for _, filter := range filters {
					relname := filepath.Base(absName)
					if ok, err := filepath.Match(filter, relname); err != nil && t.errors != nil {
						t.errors <- err
					} else if err == nil && ok {
						// we should tail that new file
						err = t.AddFile(absName)
						if err != nil && t.errors != nil {
							t.errors <- err
						}
					}
				}
			}
			dfs := t.rdirectories.RGet(absName)
			if len(dfs) > 0 {
				// a file was created in a subdirectory of one the directories that we have to recursively monitor
				for _, df := range dfs {
					if relname, err := filepath.Rel(df.directory, absName); err != nil && t.errors != nil {
						t.errors <- err
					} else if err == nil {
						if ok, err := zglob.Match(df.filter, relname); err != nil && t.errors != nil {
							t.errors <- err
						} else if err == nil && ok {
							// we should tail that new file
							err = t.AddFile(absName)
							if err != nil && t.errors != nil {
								t.errors <- err
							}
						}
					}
				}
			}
		}
	}()
	go func() {
		for err := range w.Errors {
			if t.errors != nil {
				t.errors <- err
			}
		}
	}()
	return t, nil
}

func (t *Tailor) abs(rel string) string {
	// filepath.Abs make a getcwd call each time... avoid it.
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(t.cwd, rel)
}

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

func (t *Tailor) ls(dirname string) (r []string, err error) {
	var mu sync.Mutex
	r = make([]string, 0)
	f := func(path string, typ os.FileMode) error {
		if err != nil {
			return nil
		}
		if typ.IsDir() {
			return filepath.SkipDir
		}
		mu.Lock()
		r = append(r, t.abs(path))
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
		if err != nil {
			return nil
		}
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

func (t *Tailor) AddDirectory(dirname string, filter string) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !utils.IsDir(dirname) {
		return fmt.Errorf("Not a directory: '%s'", dirname)
	}

	absName := t.abs(dirname)

	// avoid duplicates
	if t.directories.Has(absName, filter) {
		return nil
	}

	filenames, err := t.ls(absName)
	if err != nil {
		return err
	}
	err = t.watcher.Add(absName) // watch the directory for newly created files
	if err != nil {
		return err
	}
	t.directories.Add(absName, filter)

	// tail the existing files in the directory
	for _, fname := range filenames {
		relname := filepath.Base(fname)
		if ok, err := filepath.Match(filter, relname); err != nil {
			return err
		} else if ok {
			err = t.addFile(fname) // NB: we already have the t.mu lock
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Tailor) AddRecursiveDirectory(dirname string, filter string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !utils.IsDir(dirname) {
		return fmt.Errorf("Not a directory: '%s'", dirname)
	}

	absName := t.abs(dirname)

	// avoid duplicates
	if t.rdirectories.Has(absName, filter) {
		return nil
	}

	files, dirs, err := t.lsrecurse(absName)
	if err != nil {
		return err
	}
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
		if ok, err := zglob.Match(filter, relname); err != nil {
			return err
		} else if ok {
			err = t.addFile(fname)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Tailor) AddFiles(filenames []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, fname := range filenames {
		t.addFile(fname)
	}
	return nil
}

func (t *Tailor) AddFile(filename string) (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	err = t.addFile(filename)
	return err
}

func (t *Tailor) addFile(filename string) (err error) {
	// avoid duplicates
	filename = t.abs(filename)
	if _, ok := t.fspecs.Load(filename); ok {
		return nil
	}
	results := make(chan string)
	errors := make(chan error)
	ctx := context.Background()
	prefixLine(results, t.results, filename, &t.prefixWg)
	prefixErrors(errors, t.errors, filename, &t.prefixWg)
	fspec := makeFspec(ctx, filename, results, errors)
	fspec.initTail(ctx, nil, 0)
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

type dirFilter struct {
	directory string
	filter    string
}

type dirSet struct {
	set.Interface
}

func newDirSet() *dirSet {
	s := dirSet{
		Interface: set.New(set.ThreadSafe),
	}
	return &s
}

func (s *dirSet) Has(dirname string, filter string) bool {
	return s.Interface.Has(dirFilter{
		directory: dirname,
		filter:    filter,
	})
}

func (s *dirSet) Add(dirname string, filter string) {
	s.Interface.Add(dirFilter{
		directory: dirname,
		filter:    filter,
	})
}

func (s *dirSet) Remove(dirname string, filter string) {
	s.Interface.Remove(dirFilter{
		directory: dirname,
		filter:    filter,
	})
}

func (s *dirSet) Each(f func(dirname string, filter string)) {
	fbis := func(i interface{}) bool {
		df := i.(dirFilter)
		f(df.directory, df.filter)
		return true
	}
	s.Interface.Each(fbis)
}

func (s *dirSet) Get(dirname string) (filters []string) {
	filters = make([]string, 0)
	s.Each(func(dname string, filter string) {
		if dname == dirname {
			filters = append(filters, filter)
		}
	})
	return filters
}

func (s *dirSet) RGet(filename string) (dfs []dirFilter) {
	dfs = make([]dirFilter, 0)
	s.Each(func(dname string, filter string) {
		if filepath.HasPrefix(filename, dname) {
			dfs = append(dfs, dirFilter{directory: dname, filter: filter})
		}
	})
	return dfs
}
