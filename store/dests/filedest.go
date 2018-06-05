package dests

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/encoders"
	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/ctrie/filetrie"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/valyala/bytebufferpool"
)

type filesMap struct {
	fm *filetrie.Ctrie
}

func newFilesMap() *filesMap {
	return &filesMap{fm: filetrie.New(nil)}
}

func (m *filesMap) Clear() {
	m.fm.ForEach(func(e filetrie.Entry) {
		if e.Value.Release() {
			openedFilesGauge.Dec()
		}
	})
	m.fm.Clear()
}

func (m *filesMap) Put(fname string, f *utils.OFile) {
	f.Acquire()
	m.fm.Set(fname, f)
}

func (m *filesMap) Get(fname string) *utils.OFile {
	f, ok := m.fm.Lookup(fname)
	if !ok {
		return nil
	}
	f.Acquire()
	return f
}

func (m *filesMap) Remove(fname string) {
	f, ok := m.fm.Remove(fname)
	if ok {
		if f.Release() {
			openedFilesGauge.Dec()
		}
	}
}

func (m *filesMap) ForEach(f func(string, *utils.OFile)) {
	g := func(e filetrie.Entry) {
		f(e.Key, e.Value)
	}
	m.fm.ForEach(g)
}

func (m *filesMap) Filter(f func(string, *utils.OFile) bool) (ret chan *utils.OFile) {
	ret = make(chan *utils.OFile)
	ch := make(chan filetrie.Entry)
	g := func(e filetrie.Entry) bool {
		return f(e.Key, e.Value)
	}
	go func() {
		m.fm.Filter(g, ch)
		close(ch)
	}()
	go func() {
		for e := range ch {
			ret <- e.Value
		}
		close(ret)
	}()
	return ret
}

type openedFiles struct {
	files       *filesMap
	filesMu     sync.Mutex
	timeout     time.Duration
	logger      log15.Logger
	bufferSize  int
	flushPeriod time.Duration
	syncPeriod  time.Duration
	gzip        bool
	gziplevel   int
}

func newOpenedFiles(ctx context.Context, c conf.FileDestConfig, l log15.Logger) *openedFiles {
	o := openedFiles{
		files:       newFilesMap(),
		timeout:     c.OpenFileTimeout,
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
			o.files.ForEach(func(fname string, f *utils.OFile) {
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
			o.filesMu.Lock()
			for f := range o.files.Filter(func(fname string, fi *utils.OFile) bool {
				return fi.Expired()
			}) {
				o.files.Remove(f.Name)
			}
			o.filesMu.Unlock()
		}
	}()
	return &o
}

func (o *openedFiles) open(filename string) (fi *utils.OFile, err error) {
	filename, err = filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	fi = o.files.Get(filename)
	if fi != nil {
		// fastpath
		fi.Postpone(o.timeout)
		return fi, nil
	}
	dirname := filepath.Dir(filename)
	err = os.MkdirAll(dirname, 0755)
	if err != nil {
		return nil, err
	}

	o.logger.Debug("Opening file", "filename", filename)
	o.filesMu.Lock()
	defer o.filesMu.Unlock()
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	openedFilesGauge.Inc()
	fi = utils.NewOFile(f, filename, time.Now().Add(o.timeout), o.bufferSize, o.gzip, o.gziplevel, o.logger)
	o.files.Put(filename, fi)
	fi.Acquire()
	return fi, nil
}

func (o *openedFiles) closeall() {
	o.files.Clear()
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
		return encoders.EncodingError(err)
	}
	filename := strings.TrimSpace(buf.String())
	bytebufferpool.Put(buf)

	encoded, err := encoders.ChainEncode(d.encoder, message, "\n")
	if err != nil {
		d.logger.Warn("Error encoding message", "error", err)
		return encoders.EncodingError(err)
	}

	f, err := d.files.open(filename)
	if err != nil {
		d.logger.Warn("Error opening file", "filename", filename, "error", err)
		return err
	}
	_, err = io.WriteString(f, encoded)
	if f.Release() {
		openedFilesGauge.Dec()
	}
	return err
}

func (d *FileDestination) Close() error {
	d.files.closeall()
	return nil
}

func (d *FileDestination) Send(ctx context.Context, msgs []model.OutputMsg) (err eerrors.ErrorSlice) {
	return d.ForEach(ctx, d.sendOne, true, true, msgs)
}
