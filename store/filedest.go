package store

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/model"
)

type openedFile struct {
	File    *os.File
	CloseAt int64
}

type openedFiles struct {
	*sync.Map
	timeout time.Duration
	logger  log15.Logger
	nb      uint64
	max     uint64
	mu      sync.Mutex
}

func newOpenedFiles(ctx context.Context, timeout time.Duration, max uint64, logger log15.Logger) *openedFiles {
	o := openedFiles{
		Map:     &sync.Map{},
		timeout: timeout,
		logger:  logger,
		max:     max,
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			now := time.Now().Unix()
			toClose := []string{}
			o.mu.Lock()
			o.Range(func(key interface{}, val interface{}) bool {
				if atomic.LoadInt64(&(val.(*openedFile).CloseAt)) <= now {
					toClose = append(toClose, key.(string))
				}
				return true
			})
			for _, fname := range toClose {
				logger.Debug("Closing file", "filename", toClose)
				o.Delete(fname)
				atomic.AddUint64(&o.nb, ^uint64(0))
			}
			o.mu.Unlock()
		}
	}()
	return &o
}

func (o *openedFiles) open(filename string) (f *os.File, err error) {
	filename, err = filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	closeAt := now.Add(o.timeout).Unix()
	val, ok := o.Load(filename)
	if ok {
		fi := val.(*openedFile)
		f = fi.File
		atomic.StoreInt64(&fi.CloseAt, closeAt)
		fi.CloseAt = closeAt
		return
	}
	dirname := filepath.Dir(filename)
	err = os.MkdirAll(dirname, 0755)
	if err != nil {
		return nil, err
	}
	f, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	o.logger.Debug("Opening file", "filename", filename)
	o.Store(filename, &openedFile{File: f, CloseAt: closeAt})
	if atomic.AddUint64(&o.nb, 1) > o.max {
		o.closeOne()
	}
	return
}

func (o *openedFiles) closeOne() {
	var min int64 = math.MaxInt64
	var closeAt int64
	minKey := ""
	o.mu.Lock()
	o.Range(func(key interface{}, val interface{}) bool {
		closeAt = atomic.LoadInt64(&(val.(*openedFile).CloseAt))
		if closeAt < min {
			min = closeAt
			minKey = key.(string)
		}
		return true
	})
	if len(minKey) > 0 {
		o.Delete(minKey)
		atomic.AddUint64(&o.nb, ^uint64(0))
	}
	o.mu.Unlock()
}

func (o *openedFiles) closeall() {
	o.Map = &sync.Map{}
}

type fileDestination struct {
	logger       log15.Logger
	fatal        chan struct{}
	registry     *prometheus.Registry
	once         sync.Once
	ack          storeCallback
	nack         storeCallback
	permerr      storeCallback
	filenameTmpl *template.Template
	files        *openedFiles
	format       string
}

func NewFileDestination(ctx context.Context, bc conf.BaseConfig, ack, nack, permerr storeCallback, logger log15.Logger) (dest Destination, err error) {
	d := &fileDestination{
		logger:   logger,
		registry: prometheus.NewRegistry(),
		ack:      ack,
		nack:     nack,
		permerr:  permerr,
		format:   bc.FileDest.Format,
		files:    newOpenedFiles(ctx, bc.FileDest.OpenFileTimeout, bc.FileDest.OpenFilesCache, logger),
	}
	d.filenameTmpl, err = template.New("filename").Parse(bc.FileDest.Filename)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *fileDestination) Send(message model.FullMessage, partitionKey string, partitionNumber int32, topic string) (err error) {
	if len(message.Parsed.Fields.Appname) == 0 {
		message.Parsed.Fields.Appname = "empty"
	}
	buf := bytes.NewBuffer(nil)
	err = d.filenameTmpl.Execute(buf, message.Parsed)
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
	encoder, err := model.NewEncoder(f, d.format)
	if err != nil {
		err = fmt.Errorf("Error getting encoder: %s", err)
		d.permerr(message.Uid)
		return err
	}
	err = encoder.Encode(&message)
	if err == nil {
		d.ack(message.Uid)
	} else if model.IsEncodingError(err) {
		d.permerr(message.Uid)
	} else {
		d.nack(message.Uid)
	}
	return err
}

func (d *fileDestination) Close() {
	d.files.closeall()
}

func (d *fileDestination) Fatal() chan struct{} {
	return d.fatal
}

func (d *fileDestination) Gather() ([]*dto.MetricFamily, error) {
	return d.registry.Gather()
}
