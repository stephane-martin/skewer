package dests

import (
	"compress/gzip"
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/free/concurrent-writer/concurrent"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/valyala/bytebufferpool"
	"go.uber.org/atomic"
)

// ensure thread safety for the gzip writer
type cGzipWriter struct {
	w  *gzip.Writer
	mu sync.Mutex
}

func newGzipWriter(w io.Writer) *cGzipWriter {
	return &cGzipWriter{w: gzip.NewWriter(w)}
}

func (gw *cGzipWriter) Write(p []byte) (n int, err error) {
	gw.mu.Lock()
	n, err = gw.w.Write(p)
	gw.mu.Unlock()
	return
}

func (gw *cGzipWriter) Flush() (err error) {
	gw.mu.Lock()
	err = gw.w.Flush()
	gw.mu.Unlock()
	return
}

func (gw *cGzipWriter) Close() (err error) {
	gw.mu.Lock()
	err = gw.w.Close()
	gw.mu.Unlock()
	return
}

type openedFile struct {
	oFile      *os.File
	closeAt    atomic.Int64
	closed     atomic.Bool
	name       string
	writer     *concurrent.Writer
	gzipwriter *cGzipWriter
	syncmu     sync.Mutex
}

func finalizer(obj *openedFile) {
	obj.closeFile()
}

func newOpenedFile(f *os.File, name string, closeAt int64, bufferSize int, dogzip bool, level int) *openedFile {
	openedFilesGauge.Inc()
	o := openedFile{
		oFile: f,
		name:  name,
	}
	o.closeAt.Store(closeAt)
	if level == 0 || !dogzip {
		o.writer = concurrent.NewWriterAutoFlush(f, bufferSize, 0.75)
	} else {
		o.gzipwriter = newGzipWriter(f)
		o.writer = concurrent.NewWriterAutoFlush(o.gzipwriter, bufferSize, 0.75)
		runtime.SetFinalizer(&o, finalizer)
	}
	return &o
}

func (o *openedFile) Write(p []byte) (int, error) {
	// may be called concurrently
	return o.writer.Write(p)
}

func (o *openedFile) Flush() (err error) {
	// may be called concurrently
	return o.writer.Flush()
}

func (o *openedFile) Sync() (err error) {
	// may be called concurrently
	o.syncmu.Lock()
	defer o.syncmu.Unlock()
	if o.gzipwriter != nil {
		err = o.gzipwriter.Flush()
		if err != nil {
			return err
		}
	}
	return o.oFile.Sync()
}

func (o *openedFile) Closed() bool {
	return o.closed.Load()
}

func (o *openedFile) MarkClosed() (err error) {
	if o.closed.CAS(false, true) {
		// flush the concurrent cuffer
		err = o.Flush()
		if err != nil {
			return err
		}
		// sync changes to disk
		return o.Sync()
	}
	return nil
}

func (o *openedFile) closeFile() {
	if o.gzipwriter != nil {
		_ = o.gzipwriter.Close()
	}
	err := o.oFile.Close()
	if err != nil {
		openedFilesGauge.Dec()
	}
}

type openedFiles struct {
	files       *sync.Map
	filesMu     sync.Mutex
	timeout     time.Duration
	logger      log15.Logger
	nb          atomic.Uint64
	max         uint64
	bufferSize  int
	flushPeriod time.Duration
	syncPeriod  time.Duration
	gzip        bool
	gziplevel   int
}

func newOpenedFiles(ctx context.Context, c conf.FileDestConfig, l log15.Logger) *openedFiles {
	o := openedFiles{
		files:       &sync.Map{},
		timeout:     c.OpenFileTimeout,
		max:         c.OpenFilesCache,
		bufferSize:  c.BufferSize,
		flushPeriod: c.FlushPeriod,
		syncPeriod:  c.SyncPeriod,
		gzip:        c.Gzip,
		gziplevel:   c.GzipLevel,
		logger:      l,
	}
	go func() {
		// flush the buffers periodically
		lastSync := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(o.flushPeriod):
			}
			now := time.Now()
			doSync := now.Sub(lastSync) > o.syncPeriod
			o.filesMu.Lock()
			o.files.Range(func(key interface{}, val interface{}) bool {
				f := val.(*openedFile)
				err := f.Flush()
				if err != nil {
					o.logger.Error("Error flushing file destination buffers. Message loss may have occurred.", "error", err)
				}
				// sync to disk if necessary
				if doSync {
					err = f.Sync()
					if err != nil {
						o.logger.Error("Error in sync files. Message loss may have occurred.", "error", err)
					}
				}
				return true
			})
			if doSync {
				lastSync = now
			}
			o.filesMu.Unlock()
		}
	}()

	go func() {
		// every second we check if some opened files are inactive and need to be closed
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			now := time.Now().Unix()
			toClose := []*openedFile{}
			o.filesMu.Lock()
			o.files.Range(func(key interface{}, val interface{}) bool {
				f := val.(*openedFile)
				if f.closeAt.Load() <= now {
					toClose = append(toClose, f)
				}
				return true
			})
			for _, f := range toClose {
				o.logger.Debug("Closing file", "filename", f.name)
				o.files.Delete(f.name)
				err := f.MarkClosed()
				if err != nil {
					o.logger.Error("Error flushing file destination buffers. Message loss may have occurred.", "error", err)
				}
				o.nb.Add(^uint64(0))
			}
			o.filesMu.Unlock()
			runtime.GC()
		}
	}()
	return &o
}

func (o *openedFiles) open(filename string) (fi *openedFile, err error) {
	filename, err = filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	closeAt := now.Add(o.timeout).Unix()
	val, ok := o.files.Load(filename)
	if ok {
		fi = val.(*openedFile)
		fi.closeAt.Store(closeAt)
		return
	}
	dirname := filepath.Dir(filename)
	err = os.MkdirAll(dirname, 0755)
	if err != nil {
		return nil, err
	}

	runtime.GC() // ensure that the file is really closed before reopening it
	o.logger.Debug("Opening file", "filename", filename)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	fi = newOpenedFile(f, filename, closeAt, o.bufferSize, o.gzip, o.gziplevel)
	o.files.Store(filename, fi)
	if o.nb.Add(1) > o.max {
		// if there are too many opened files, close one of them
		o.closeOne()
	}
	return
}

func (o *openedFiles) closeOne() {
	var min int64 = math.MaxInt64
	var minf *openedFile
	o.filesMu.Lock()
	defer o.filesMu.Unlock()
	o.files.Range(func(key interface{}, val interface{}) bool {
		f := val.(*openedFile)
		closeAt := f.closeAt.Load()
		if closeAt < min {
			min = closeAt
			minf = f
		}
		return true
	})
	if minf != nil {
		o.files.Delete(minf.name)
		err := minf.MarkClosed()
		if err != nil {
			o.logger.Error("Error flushing file destination buffers. Message loss may have occurred.", "error", err)
		}
		o.nb.Add(^uint64(0))
	}
}

func (o *openedFiles) closeall() {
	fnames := []string{}
	o.files.Range(func(key interface{}, val interface{}) bool {
		f := val.(*openedFile)
		fnames = append(fnames, key.(string))
		err := f.MarkClosed()
		if err != nil {
			o.logger.Error("Error flushing file destination buffers. Message loss may have occurred.", "error", err)
		}
		return true
	})
	// force the finalizers to run
	for _, fname := range fnames {
		o.files.Delete(fname)
	}
	o.files = &sync.Map{}
	runtime.GC()
}

type FileDestination struct {
	*baseDestination
	filenameTmpl *template.Template
	files        *openedFiles
}

func NewFileDestination(ctx context.Context, e *Env) (Destination, error) {
	dest := &FileDestination{
		baseDestination: newBaseDestination(conf.File, "file", e),
		files:           newOpenedFiles(ctx, e.config.FileDest, e.logger),
	}
	err := dest.setFormat(e.config.FileDest.Format)
	if err != nil {
		return nil, err
	}
	fname := e.config.FileDest.Filename
	if e.confined {
		fname = filepath.Join("/tmp", "filedest", fname)
	}
	dest.filenameTmpl, err = template.New("filename").Parse(fname)
	if err != nil {
		return nil, err
	}
	return dest, nil
}

func (d *FileDestination) sendOne(ctx context.Context, message *model.FullMessage) (err error) {
	if len(message.Fields.AppName) == 0 {
		message.Fields.AppName = "unknown"
	}
	buf := bytebufferpool.Get()
	err = d.filenameTmpl.Execute(buf, message.Fields)
	if err != nil {
		d.logger.Warn("Error calculating filename", "error", err)
		bytebufferpool.Put(buf)
		return encoders.EncodingError(err)
	}
	filename := strings.TrimSpace(buf.String())
	bytebufferpool.Put(buf)
	f, err := d.files.open(filename)
	if err != nil {
		d.logger.Warn("Error opening file", "filename", filename, "error", err)
		return err
	}
	encoded, err := encoders.ChainEncode(d.encoder, message, "\n")
	if err != nil {
		d.logger.Warn("Error encoding message", "error", err)
		return encoders.EncodingError(err)
	}
	_, err = io.WriteString(f, encoded)
	if err != nil {
		return err
	}
	if f.Closed() {
		err = f.MarkClosed()
		if err == nil {
			return nil
		}
		return err
	}
	return nil
}

func (d *FileDestination) Close() error {
	d.files.closeall()
	return nil
}

func (d *FileDestination) Send(ctx context.Context, msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err eerrors.ErrorSlice) {
	return d.ForEach(ctx, d.sendOne, true, true, msgs)
}
