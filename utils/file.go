package utils

import (
	"compress/gzip"
	"io"
	"os"
	"sync"
	"time"

	"github.com/free/concurrent-writer/concurrent"
	"github.com/inconshreveable/log15"
	"go.uber.org/atomic"
)

type OFile struct {
	oFile      *os.File
	closeAt    atomic.Int64
	Name       string
	writer     *concurrent.Writer
	gzipwriter *cGzipWriter
	syncmu     sync.Mutex
	refs       atomic.Int32
	logger     log15.Logger
}

func NewOFile(f *os.File, name string, closeAt time.Time, bufferSize int, doGzip bool, gzipLevel int, logger log15.Logger) *OFile {
	//openedFilesGauge.Inc()
	o := &OFile{
		oFile:  f,
		Name:   name,
		logger: logger,
	}
	o.closeAt.Store(closeAt.UnixNano())
	if gzipLevel == 0 || !doGzip {
		o.writer = concurrent.NewWriterAutoFlush(f, bufferSize, 0.75)
		// the native go file finalizer will close the file when we do not reference it anymore
	} else {
		o.gzipwriter = newGzipWriter(f)
		o.writer = concurrent.NewWriterAutoFlush(o.gzipwriter, bufferSize, 0.75)
	}
	return o
}

func (o *OFile) Release() {
	if o.refs.Add(-1) == 0 {
		o.closeFile()
		o.logger.Debug("Closing file", "filename", o.Name)
	}
}

func (o *OFile) Acquire() {
	o.refs.Add(1)
}

func (o *OFile) Postpone(d time.Duration) {
	o.closeAt.Store(time.Now().Add(d).UnixNano())
}

func (o *OFile) Expired() bool {
	return time.Now().After(time.Unix(0, o.closeAt.Load()))
}

func (o *OFile) Write(p []byte) (int, error) {
	// may be called concurrently
	return o.writer.Write(p)
}

func (o *OFile) Flush() (err error) {
	return o.writer.Flush()
}

func (o *OFile) Sync() (err error) {
	// may be called concurrently
	o.syncmu.Lock()
	defer o.syncmu.Unlock()
	if o.gzipwriter != nil {
		// flush the gzip writer
		err = o.gzipwriter.Flush()
		if err != nil {
			return err
		}
	}
	// fsync the file itself
	return o.oFile.Sync()
}

func (o *OFile) closeFile() {
	o.Flush()
	if o.gzipwriter != nil {
		_ = o.gzipwriter.Close()
	}
	err := o.oFile.Close()
	if err != nil {
		//openedFilesGauge.Dec()
	}
}

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
