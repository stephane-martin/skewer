package tail

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

type fileSpec struct {
	sync.Mutex
	name           string
	size           int64
	mtime          time.Time
	dev            uint64
	ino            uint64
	mode           os.FileMode
	ignore         bool
	remote         bool
	tailable       bool
	file           *os.File
	err            error
	unchangedStats uint8
	writer         *resultLines
	errors         chan error
}

func makeFspec(ctx context.Context, filename string, results chan string, errors chan error) *fileSpec {
	return &fileSpec{
		name:   filename,
		writer: makeWriter(ctx, results),
		errors: errors,
	}
}

type fileSpecs []*fileSpec

func (fs fileSpecs) sort() (notifyFS fileSpecs, classicalFS fileSpecs) {
	if len(fs) == 0 {
		return make([]*fileSpec, 0), make([]*fileSpec, 0)
	}
	notifyFS = make([]*fileSpec, 0, len(fs))
	classicalFS = make([]*fileSpec, 0, len(fs))
	for _, f := range fs {
		if f.hasClassicalFollow() {
			classicalFS = append(classicalFS, f)
		} else {
			notifyFS = append(notifyFS, f)
		}
	}
	return notifyFS, classicalFS
}

func (f *fileSpec) close() {
	f.Lock()
	defer f.Unlock()
	if f.name != "-" && f.file != nil {
		f.file.Close()
		f.file = nil
	}
	if f.writer != nil {
		f.writer.Close()
		f.writer = nil
	}
	if f.errors != nil {
		close(f.errors)
		f.errors = nil
	}
}

func (f *fileSpec) hasClassicalFollow() bool {
	return f.isStdin() || f.isRemote() || f.isSymlink() || (!f.isRegular() && !f.isFIFO())
}

func (f *fileSpec) isRemote() bool {
	return f.file != nil && f.remote
}

func (f *fileSpec) isNonRemote() bool {
	return f.file != nil && !f.remote
}

func (f *fileSpec) isSymlink() bool {
	var err error
	var infos os.FileInfo
	if len(f.name) > 0 && f.name != "-" {
		infos, err = os.Lstat(f.name)
		if err == nil {
			return isLink(infos.Mode())
		}
	}
	return false
}

func (f *fileSpec) isRegular() bool {
	return f.file != nil && isRegular(f.mode)
}

func (f *fileSpec) isFIFO() bool {
	return f.file != nil && isFIFO(f.mode)
}

func (f *fileSpec) isStdin() bool {
	return !f.ignore && f.name == "-"
}

var maxUnchangedStat uint8 = 5

func getIno(infos os.FileInfo) uint64 {
	if stat, ok := infos.Sys().(*syscall.Stat_t); ok {
		return stat.Ino
	}
	panic("Can't type assert FileInfo to Stat_t")
}

func getDev(infos os.FileInfo) uint64 {
	if stat, ok := infos.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Dev)
	}
	panic("Can't type assert FileInfo to Stat_t")
}

func (f *fileSpec) record(file *os.File, size int64, infos os.FileInfo) {
	f.file = file
	f.size = size
	f.mtime = infos.ModTime()
	f.mode = infos.Mode()
	f.unchangedStats = 0
	f.ignore = false
	f.dev = getDev(infos)
	f.ino = getIno(infos)
}

func (f *fileSpec) print() {
	f.Lock()
	defer f.Unlock()
	if f == nil {
		return
	}
	if f.file == nil {
		return
	}
	if f.ignore {
		return
	}

	stats, err := f.file.Stat()
	if err != nil {
		f.err = err
		f.file.Close()
		f.file = nil
		if f.errors != nil {
			f.errors <- err
		}
		return
	}

	if isRegular(stats.Mode()) && stats.Size() < f.size {
		// truncated
		f.file.Seek(0, 0)
		f.size = 0
	} else if isRegular(stats.Mode()) && stats.Size() == f.size && stats.ModTime() == f.mtime {
		return
	}

	n, err := io.Copy(f.writer, f.file)
	f.size += n

	if err != nil && f.errors != nil {
		f.errors <- err
	}
}

func (f *fileSpec) recheck(notifyMode bool) {
	f.Lock()
	defer f.Unlock()
	var file *os.File
	var err error
	var newInfos os.FileInfo
	ok := true
	prevErr := f.err
	file, err = os.Open(f.name)
	if err != nil {
		// can't open file
		ok = false
		f.err = err
		f.tailable = false
		file = nil
	} else {
		newInfos, err = os.Lstat(f.name)
		if err != nil {
			ok = false
			f.err = err
		} else {
			if isLink(newInfos.Mode()) {
				ok = false
				f.err = fmt.Errorf("File '%s' is a symbolic link", f.name)
				f.ignore = true
			} else {
				newInfos, err = os.Stat(f.name)
				if err != nil {
					ok = false
					f.err = err
				} else if !isTailable(newInfos.Mode()) {
					ok = false
					f.err = fmt.Errorf("File '%s' is not tailable", f.name)
					f.tailable = false
					// f.ignore = !follow_mode == Follow_name
					f.ignore = false
				} else if notifyMode {
					remote, err := fremote(file)
					if err != nil {
						ok = false
						f.err = err
						f.ignore = true
						f.remote = true
					} else if remote {
						ok = false
						f.err = fmt.Errorf("File '%s' is on a remote FS", f.name)
						f.ignore = true
						f.remote = true
					} else {
						f.err = nil
					}
				} else {
					f.err = nil
				}
			}
		}
	}

	newFile := false

	if !ok {
		if file != nil {
			file.Close()
		}
		if f.file != nil {
			f.file.Close()
			f.file = nil
		}
	} else if os.IsNotExist(prevErr) {
		newFile = true
	} else if f.file == nil {
		newFile = true
	} else if f.ino != getIno(newInfos) || f.dev != getDev(newInfos) {
		// File has been replaced (e.g., via log rotation)
		newFile = true
		// close previous file
		if f.file != nil {
			f.file.Close()
			f.file = nil
		}
	} else {
		// No changes detected, so close new file
		if file != nil {
			file.Close()
		}
	}

	if newFile {
		f.record(file, 0, newInfos)
		file.Seek(0, 0)
	}

	if f.errors != nil && f.err != nil {
		f.errors <- f.err
	}

	return
}

func (fspec *fileSpec) initTail(ctx context.Context, pwg *sync.WaitGroup, nbLines int) {
	if pwg != nil {
		defer pwg.Done()
	}
	var file *os.File
	var infos os.FileInfo

	var err error
	if fspec.name == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(fspec.name)
	}

	if err != nil {
		fspec.tailable = false
		fspec.file = nil
		fspec.err = err
		fspec.ignore = false
		fspec.ino = 0
		fspec.dev = 0
	} else {
		infos, err = file.Stat()
		if err != nil {
			fspec.err = err
			fspec.ignore = false
			fspec.file = nil
			file.Close()
		} else if !isTailable(infos.Mode()) {
			fspec.err = fmt.Errorf("The file is not tailable")
			fspec.tailable = false
			fspec.ignore = false
			fspec.file = nil
			file.Close()
		} else {

			var readPos int64
			tailFinished := make(chan struct{})
			go func() {
				readPos, err = selectTailFunc(file, infos)(nbLines, fspec.writer)
				close(tailFinished)
			}()
			select {
			case <-ctx.Done():
				file.Close()
				return
			case <-tailFinished:
			}
			if err != nil {
				fspec.err = err
				fspec.ignore = false
				fspec.file = nil
				file.Close()
			} else {
				infos, err = file.Stat()
				if err != nil {
					fspec.err = err
					fspec.ignore = false
					fspec.file = nil
					file.Close()
				} else {
					//fmt.Fprintln(os.Stderr, "readPos", readPos)
					fspec.record(file, readPos, infos)
					remote, err := fremote(file)
					if err != nil {
						fspec.remote = true
					} else {
						fspec.remote = remote
					}
				}
			}
		}
	}
	if fspec.err != nil && fspec.errors != nil {
		fspec.errors <- fspec.err
	}

}
