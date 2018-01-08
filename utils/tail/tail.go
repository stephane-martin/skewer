package tail

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

const bufferSize = 512 * 8

var lineEnd byte = '\n'
var lineEndS = []byte{lineEnd}
var lineEndString = "\n"

func isCharDevice(mode os.FileMode) bool {
	return (mode & os.ModeCharDevice) != 0
}

func isRegular(mode os.FileMode) bool {
	return mode.IsRegular()
}

func isFIFO(mode os.FileMode) bool {
	return (mode & os.ModeNamedPipe) != 0
}

func isSocket(mode os.FileMode) bool {
	return (mode & os.ModeSocket) != 0
}

func isLink(mode os.FileMode) bool {
	return (mode & os.ModeSymlink) != 0
}

func isDevice(mode os.FileMode) bool {
	return (mode & os.ModeDevice) != 0
}

func isTailable(mode os.FileMode) bool {
	return isRegular(mode) || isFIFO(mode) || isCharDevice(mode) || isSocket(mode)
}

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

func fremote(f *os.File) (b bool, err error) {
	stats := syscall.Statfs_t{}
	err = syscall.Fstatfs(int(f.Fd()), &stats)
	if err != nil {
		return
	}
	/*
		fsname := make([]byte, len(stats.Fstypename))
		for i, c := range stats.Fstypename {
			fsname[i] = byte(c)
		}
		fmt.Fprintln(os.Stderr, string(fsname))
	*/
	b = !isLocalFS(int64(stats.Type))
	return
}

func removeNLChans(ctx context.Context, input chan string, output chan string) {
	go func() {
		defer func() {
			if output != nil {
				close(output)
			}
		}()
		var line string
		var more bool
		for {
			select {
			case <-ctx.Done():
				return
			case line, more = <-input:
				if more {
					if output != nil {
						output <- strings.Trim(line, lineEndString)
					}
				} else {
					return
				}
			}
		}
	}()
}

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

type TailFileOpts struct {
	filename string
	nbLines  int
	results  chan string
	errors   chan error
	wg       *sync.WaitGroup
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
}

type TailFilesOpt func(opts *TailFilesOpts)

func (opts *TailFilesOpts) Filenames() (r []string) {
	r = make([]string, 0, len(opts.filenames))
	for fname := range opts.filenames {
		r = append(r, fname)
	}
	sort.Strings(r)
	return r
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

func TailFiles(ctx context.Context, opts ...TailFilesOpt) {
	env := TailFilesOpts{
		nbLines: 10,
	}

	for _, opt := range opts {
		opt(&env)
	}

	filenames := env.Filenames()
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
		resultsChan := make(chan string)
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

func TailFile(ctx context.Context, opts ...TailFileOpt) (err error) {
	env := TailFileOpts{
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
	o := make(chan string)
	r := &resultLines{
		output: o,
		buf:    bytes.NewBuffer(nil),
	}
	removeNLChans(ctx, o, env.results)

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
			close(o) // makes removeNLChans close env.results if needed
		}()
		var err error
		tailFinished := make(chan struct{})
		go func() {
			_, err = dotail(env.nbLines, r)
			close(tailFinished)
		}()

		select {
		case <-tailFinished:
		case <-ctx.Done():
			return
		}

		if r.buf.Len() > 0 {
			o <- r.buf.String()
		}
		if err != nil && env.errors != nil {
			env.errors <- err
		}

	}()
	return nil
}

type resultLines struct {
	output chan string
	buf    *bytes.Buffer
}

func (r *resultLines) Write(p []byte) (int, error) {
	lorig := len(p)
	if r == nil {
		return lorig, nil
	}
	if r.output == nil {
		return lorig, nil
	}
	if lorig == 0 {
		return 0, nil
	}
	var l int
	for {
		l = len(p)
		if l == 0 {
			return lorig, nil
		}
		idx := bytes.Index(p, lineEndS)
		if idx == -1 {
			r.buf.Write(p)
			return lorig, nil
		}
		r.buf.Write(p[0 : idx+1])
		r.output <- r.buf.String()
		r.buf = bytes.NewBuffer(nil)
		if idx == l-1 {
			return lorig, nil
		}
		p = p[idx+1:]
	}
}

func fileLines(file *os.File, nbLines int, startPos, endPos int64, output io.Writer) (readPos int64, err error) {
	if nbLines <= 0 {
		return
	}
	buf := make([]byte, bufferSize)
	pos := endPos
	bytesRead := int((pos - startPos) % bufferSize)
	if bytesRead == 0 {
		bytesRead = bufferSize
	}
	pos -= int64(bytesRead)

	file.Seek(pos, 0)
	bytesRead, err = io.ReadFull(file, buf[:bytesRead])
	readPos = pos + int64(bytesRead)
	if err != nil {
		return
	}

	if buf[bytesRead-1] != lineEnd {
		nbLines--
	}

	for bytesRead > 0 {
		n := bytesRead

		for n > 0 {
			n = bytes.LastIndex(buf[:n], lineEndS)
			if n == -1 {
				break
			}
			if nbLines == 0 {
				l := bytesRead - (n + 1)
				if l > 0 {
					output.Write(buf[n+1 : n+1+l])
				}
				l2 := int64(endPos - (pos + int64(bytesRead)))
				if l > 0 {
					var n int64
					n, err = io.CopyN(output, file, l2)
					readPos += n
				}
				return
			}
			nbLines--
		}

		if pos == startPos {
			file.Seek(startPos, 0)
			var n int64
			n, err = io.Copy(output, file)
			readPos += n
			return
		}
		pos -= bufferSize
		file.Seek(pos, 0)
		bytesRead, err = io.ReadFull(file, buf)
		readPos = pos + int64(bytesRead)
		if err != nil {
			return
		}
	}

	return
}

type lineBuffer struct {
	buffer [bufferSize]byte
	nBytes int
	nLines int
	next   *lineBuffer
}

func pipeLines(file *os.File, nbLines int, output io.Writer) (readPos int64, err error) {
	if nbLines <= 0 {
		return
	}
	var totalLines int
	var nRead int

	first := &lineBuffer{}
	tmp := &lineBuffer{}
	last := first

	for {
		nRead, err = file.Read(tmp.buffer[:])
		readPos += int64(nRead)
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		tmp.nBytes = nRead
		tmp.nLines = bytes.Count(tmp.buffer[:tmp.nBytes], lineEndS)
		tmp.next = nil
		totalLines += tmp.nLines

		if (tmp.nBytes + last.nBytes) < bufferSize {
			copy(last.buffer[last.nBytes:], tmp.buffer[:tmp.nBytes])
			last.nBytes += tmp.nBytes
			last.nLines += tmp.nLines
		} else {
			last.next = tmp
			last = tmp
			if (totalLines - first.nLines) > nbLines {
				tmp = first
				totalLines -= first.nLines
				first = first.next
			} else {
				tmp = &lineBuffer{}
			}
		}
	}
	// when nbLines == 0, we exhaust the pipe then return
	if last.nBytes == 0 || nbLines == 0 {
		return
	}

	if last.buffer[last.nBytes-1] != lineEnd {
		last.nLines++
		totalLines++
	}

	tmp = first
	for (totalLines - tmp.nLines) > nbLines {
		totalLines -= tmp.nLines
		tmp = tmp.next
	}
	beg := tmp.buffer[:tmp.nBytes]
	j := totalLines - nbLines
	// We made sure that 'totalLines - nbLines <= tmp.nLines'
	for j > 0 {
		idx := bytes.Index(beg, lineEndS)
		if idx == -1 {
			panic("should not happen")
		}
		beg = beg[idx+1:]
		j--
	}
	output.Write(beg)

	tmp = tmp.next
	for tmp != nil {
		output.Write(tmp.buffer[:tmp.nBytes])
		tmp = tmp.next
	}
	return
}

func step(fspec *fileSpec, blockingStdin bool) (bool, error) {
	if fspec.file == nil {
		fspec.recheck(false)
		return false, nil
	}

	mode := fspec.mode
	var infos os.FileInfo
	var err error

	if !blockingStdin {
		infos, err = fspec.file.Stat()
		if err != nil {
			fspec.err = err
			fspec.file.Close()
			fspec.file = nil
			return false, err
		}

		if infos.ModTime() == fspec.mtime && infos.Mode() == fspec.mode && (!isRegular(fspec.mode) || infos.Size() == fspec.size) {
			if fspec.unchangedStats >= maxUnchangedStat {
				fspec.recheck(false)
				fspec.unchangedStats = 0
			} else {
				fspec.unchangedStats++
			}
			return false, nil
		}

		fspec.mtime = infos.ModTime()
		fspec.mode = infos.Mode()
		fspec.unchangedStats = 0

		if isRegular(mode) && fspec.size > infos.Size() {
			// truncated
			fspec.file.Seek(0, 0)
			fspec.size = 0
		}
	}

	var nread int64
	if blockingStdin {
		nread, err = io.CopyN(fspec.writer, fspec.file, bufferSize)
	} else if isRegular(mode) && fspec.remote {
		nread, err = io.CopyN(fspec.writer, fspec.file, infos.Size()-fspec.size)
		if err == io.EOF {
			err = nil
		}
	} else {
		nread, err = io.Copy(fspec.writer, fspec.file)
		if err == io.EOF {
			err = nil
		}
	}
	fspec.size += nread
	return nread > 0, err

}

func followClassical(ctx context.Context, wg *sync.WaitGroup, fspec *fileSpec, sleepInterval time.Duration) {
	defer wg.Done()
	blockingStdin := false
	if fspec.file != nil && fspec.name == "-" {
		infos, err := fspec.file.Stat()
		if err == nil {
			if !isRegular(infos.Mode()) {
				blockingStdin = true
			} else {
			}
		}
	}
Loop:
	for {
		if fspec.ignore {
			return
		}

		havenew, err := step(fspec, blockingStdin)

		if err == io.EOF {
			return
		} else if err != nil && fspec.errors != nil {
			fspec.errors <- err
		}

		if havenew {
			select {
			case <-ctx.Done():
				return
			default:
				continue Loop
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(sleepInterval):
			continue Loop
		}
	}
}

func followNotify(ctx context.Context, pwg *sync.WaitGroup, fspecs fileSpecs, watcherErrors chan error) {
	defer pwg.Done()
	m := map[string]bool{}
	fspecsMap := map[string]*fileSpec{}
	for _, fspec := range fspecs {
		absName, err := filepath.Abs(fspec.name)
		if err != nil {
			if fspec.errors != nil {
				fspec.errors <- err
			}
			return
		}
		m[filepath.Dir(fspec.name)] = true
		fspecsMap[absName] = fspec
	}
	directories := make([]string, 0, len(fspecs))
	for dname := range m {
		directories = append(directories, dname)
	}

	var wg sync.WaitGroup
	dirWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	for _, dname := range directories {
		dirWatcher.Add(dname)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		var ev fsnotify.Event
		var fspec *fileSpec
		var ok bool
		var once sync.Once

		for ev = range dirWatcher.Events {
			once.Do(func() {
				for _, fspec = range fspecsMap {
					fspec.recheck(true)
					fspec.print()
				}
			})
			absName, err := filepath.Abs(ev.Name)
			if err != nil {
				if watcherErrors != nil {
					watcherErrors <- err
				}
				continue
			}
			//fmt.Fprintln(os.Stderr, "watcher", "filename", absName, "operation", ev.Op)
			if fspec, ok = fspecsMap[absName]; ok {
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

	wg.Add(1)
	go func() {
		for err := range dirWatcher.Errors {
			if watcherErrors != nil {
				watcherErrors <- err
			}
		}
		wg.Done()
	}()

	<-ctx.Done()
	dirWatcher.Close()
	wg.Wait()
}

func FollowFiles(ctx context.Context, sleepInterval time.Duration, opts ...TailFilesOpt) {
	var wg sync.WaitGroup
	if sleepInterval == 0 {
		sleepInterval = time.Second
	}
	env := TailFilesOpts{
		nbLines: 10,
	}

	for _, opt := range opts {
		opt(&env)
	}

	if env.errors != nil {
		defer func() {
			wg.Wait()
			close(env.errors)
		}()
	}

	if env.results != nil {
		defer func() {
			wg.Wait()
			close(env.results)
		}()
	}

	filenames := env.Filenames()
	if len(filenames) == 0 {
		close(env.results)
		return
	}

	var fspecs fileSpecs = make([]*fileSpec, 0, len(filenames))
	var initTailWg sync.WaitGroup

	for _, filename := range filenames {
		fname := filename

		errChan := make(chan error)
		defer close(errChan)

		o := make(chan string)
		w := &resultLines{
			output: o,
			buf:    bytes.NewBuffer(nil),
		}
		defer close(o) // will close resultsChan

		resultsChan := make(chan string)
		removeNLChans(ctx, o, resultsChan)
		prefixLine(resultsChan, env.results, fname, &wg)
		prefixErrors(errChan, env.errors, fname, &wg)

		fspec := &fileSpec{
			name:   fname,
			writer: w,
			errors: errChan,
		}
		fspecs = append(fspecs, fspec)
		initTailWg.Add(1)
		go initTail(ctx, &initTailWg, fspec, env.nbLines)
	}

	defer func() {
		for _, fspec := range fspecs {
			if fspec.name != "-" && fspec.file != nil {
				fspec.file.Close()
			}
		}
	}()

	initTailWg.Wait()
	select {
	case <-ctx.Done():
		return
	default:
	}

	notifySpecs, classicalSpecs := fspecs.sort()

	var followWg sync.WaitGroup
	for _, fspec := range classicalSpecs {
		followWg.Add(1)
		go followClassical(ctx, &followWg, fspec, sleepInterval)
	}
	followWg.Add(1)
	go followNotify(ctx, &followWg, notifySpecs, env.errors)

	followWg.Wait()

	for _, fspec := range fspecs {
		if env.results != nil && fspec.writer.buf.Len() > 0 {
			env.results <- FileLine{
				Line:     strings.Trim(fspec.writer.buf.String(), lineEndString),
				Filename: fspec.name,
			}
		}
	}

}

func FollowFile(ctx context.Context, sleepInterval time.Duration, opts ...TailFileOpt) {
	if sleepInterval == 0 {
		sleepInterval = time.Second
	}
	env := TailFileOpts{
		nbLines: 10,
	}

	for _, opt := range opts {
		opt(&env)
	}
	if env.errors != nil {
		defer close(env.errors)
	}

	o := make(chan string)
	w := &resultLines{
		output: o,
		buf:    bytes.NewBuffer(nil),
	}
	removeNLChans(ctx, o, env.results)
	defer close(o) // will close env.results

	if len(env.filename) == 0 {
		return
	}

	fspec := &fileSpec{
		name:   env.filename,
		writer: w,
		errors: env.errors,
	}

	defer func() {
		if fspec.name != "-" && fspec.file != nil {
			fspec.file.Close()
		}
	}()

	initTail(ctx, nil, fspec, env.nbLines)

	select {
	case <-ctx.Done():
		return
	default:
	}

	var followWg sync.WaitGroup
	followWg.Add(1)
	if fspec.hasClassicalFollow() {
		go followClassical(ctx, &followWg, fspec, sleepInterval)
	} else {
		go followNotify(ctx, &followWg, []*fileSpec{fspec}, env.errors)
	}
	followWg.Wait()
	if env.results != nil && fspec.writer.buf.Len() > 0 {
		if env.results != nil {
			env.results <- strings.Trim(fspec.writer.buf.String(), lineEndString)
		}
	}
}

func initTail(ctx context.Context, pwg *sync.WaitGroup, fspec *fileSpec, nbLines int) {
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
				// when context is canceled, removeNLChans closes env.results
				// so we just have to close env.errors
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
