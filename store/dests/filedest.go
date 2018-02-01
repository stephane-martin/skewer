package dests

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/free/concurrent-writer/concurrent"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/model/encoders"
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
	closeAt    int64
	closed     int32
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
		oFile:   f,
		name:    name,
		closeAt: closeAt,
	}
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
	return atomic.LoadInt32(&o.closed) == 1
}

func (o *openedFile) MarkClosed() (err error) {
	atomic.StoreInt32(&o.closed, 1)
	// flush the concurrent cuffer
	err = o.Flush()
	if err != nil {
		return err
	}
	// sync changes to disk
	return o.Sync()
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
	nb          uint64
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
				if atomic.LoadInt64(&(f.closeAt)) <= now {
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
				atomic.AddUint64(&o.nb, ^uint64(0))
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
		atomic.StoreInt64(&fi.closeAt, closeAt)
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
	if atomic.AddUint64(&o.nb, 1) > o.max {
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
		closeAt := atomic.LoadInt64(&f.closeAt)
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
		atomic.AddUint64(&o.nb, ^uint64(0))
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

func (d *FileDestination) sendOne(message *model.FullMessage) (err error) {
	defer model.FullFree(message)

	if len(message.Fields.AppName) == 0 {
		message.Fields.AppName = "unknown"
	}
	buf := bytes.NewBuffer(nil)
	err = d.filenameTmpl.Execute(buf, message.Fields)
	if err != nil {
		err = fmt.Errorf("Error calculating filename: %s", err)
		d.permerr(message.Uid)
		return err
	}
	filename := strings.TrimSpace(buf.String())
	f, err := d.files.open(filename)
	if err != nil {
		err = fmt.Errorf("Error opening file '%s': %s", filename, err)
		d.nack(message.Uid)
		return err
	}
	encoded, err := encoders.ChainEncode(d.encoder, message, "\n")
	if err != nil {
		d.permerr(message.Uid)
		return err
	}
	_, err = f.Write(encoded)
	if err != nil {
		d.nack(message.Uid)
		return err
	}
	if f.Closed() {
		err = f.MarkClosed()
		if err == nil {
			d.ack(message.Uid)
			return nil
		}
		d.nack(message.Uid)
		return err
	}
	d.ack(message.Uid)
	return nil
}

func (d *FileDestination) Close() error {
	d.files.closeall()
	return nil
}

func (d *FileDestination) Send(msgs []model.OutputMsg, partitionKey string, partitionNumber int32, topic string) (err error) {
	var i int
	var e error
	for i = range msgs {
		e = d.sendOne(msgs[i].Message)
		if e != nil {
			err = e
		}
	}
	return err
}
