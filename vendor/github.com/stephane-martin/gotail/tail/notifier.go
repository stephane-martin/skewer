package tail

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/fatih/set"
	"github.com/fsnotify/fsnotify"
)

type notifier struct {
	fspecsMap   *dir2fspec
	errors      chan error
	directories set.Interface
	wg          sync.WaitGroup
	mu          sync.Mutex
	watcher     *fsnotify.Watcher
	cwd         string
}

func newNotifier(errors chan error) (n *notifier, err error) {
	n = &notifier{
		errors:      errors,
		directories: set.New(set.ThreadSafe),
		fspecsMap:   newMap(),
	}
	n.cwd, err = os.Getwd()
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (n *notifier) abs(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(n.cwd, rel)
}

func (n *notifier) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	n.watcher = w
	n.follow()
	return nil
}

func (n *notifier) Stop() (err error) {
	err = n.watcher.Close()
	if err != nil {
		return err
	}
	n.wg.Wait()
	return nil
}

func (n *notifier) addFile(fspec *fileSpec) error {
	// add fspec to fspecmap
	absName, err := filepath.Abs(fspec.name)
	if err != nil {
		return err
	}
	if n.fspecsMap.Load(absName) != nil {
		// the fspec was already added
		return nil
	}
	n.fspecsMap.Store(absName, fspec)
	dirname := filepath.Dir(absName)
	if !n.directories.Has(dirname) {
		n.directories.Add(dirname)
		if n.watcher != nil {
			// the watcher is already running
			n.watcher.Add(dirname)
		}
	}
	return nil
}

func (n *notifier) AddFile(fspec *fileSpec) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.addFile(fspec)
}

func (n *notifier) AddFiles(fspecs fileSpecs) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	var err error
	for _, fspec := range fspecs {
		err = n.addFile(fspec)
		if err != nil {
			return err
		}
	}
	return nil
}

func (n *notifier) follow() {
	n.directories.Each(func(dnameIface interface{}) bool {
		dname := dnameIface.(string)
		n.watcher.Add(dname)
		return true
	})

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		seen := map[string]bool{}

		for ev := range n.watcher.Events {
			absName := n.abs(ev.Name)
			//fmt.Fprintln(os.Stderr, "watcher", "filename", absName, "operation", ev.Op)
			fspec := n.fspecsMap.Load(absName)
			if fspec != nil {
				// the first time we see a watcher event concerning a particular fspec
				// we need to make sure that the fspec is up to date
				if !seen[absName] {
					seen[absName] = true
					fspec.recheck(true)
					fspec.print()
				}
				switch ev.Op {
				case fsnotify.Write:
					fspec.print()
				case fsnotify.Create:
					fspec.recheck(true)
					fspec.print()
				case fsnotify.Remove:
					fspec.recheck(true)
				case fsnotify.Rename:
					fspec.recheck(true)
				}
			}
		}
	}()

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		for err := range n.watcher.Errors {
			if n.errors != nil {
				n.errors <- err
			}
		}
	}()

}

type dir2fspec struct {
	smap *sync.Map
}

func newMap() *dir2fspec {
	m := dir2fspec{smap: &sync.Map{}}
	return &m
}

func (m *dir2fspec) Load(dirname string) *fileSpec {
	if spec, ok := m.smap.Load(dirname); ok {
		return spec.(*fileSpec)
	}
	return nil
}

func (m *dir2fspec) Store(dirname string, fspec *fileSpec) bool {
	if _, ok := m.smap.Load(dirname); ok {
		return false
	}
	m.smap.Store(dirname, fspec)
	return true
}

func (m *dir2fspec) ForEach(f func(dirname string, fspec *fileSpec)) {
	g := func(d interface{}, fs interface{}) bool {
		dirname := d.(string)
		fspec := fs.(*fileSpec)
		f(dirname, fspec)
		return true
	}
	m.smap.Range(g)
}
