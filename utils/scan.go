package utils

import (
	"bufio"
	"context"
	"strings"
	"sync"

	"github.com/stephane-martin/skewer/utils/eerrors"
)

type Scanner interface {
	Scan() bool
	Err() error
	Bytes() []byte
	Text() string
	Buffer([]byte, int)
	Split(bufio.SplitFunc)
}

type CtxScanner struct {
	scanner Scanner
	reschan chan bool
	sync.Mutex
	ctx context.Context
	sync.Once
}

type RecoverScanner struct {
	scanner    Scanner
	recovererr error
}

func WithContext(ctx context.Context, scanner Scanner) Scanner {
	if ctx == nil {
		return scanner
	}
	return &CtxScanner{
		scanner: scanner,
		ctx:     ctx,
		reschan: make(chan bool),
	}
}

func WithRecover(scanner Scanner) *RecoverScanner {
	return &RecoverScanner{scanner: scanner}
}

func (s *CtxScanner) init() {
	if s.ctx == nil {
		return
	}
	s.Lock()
	go func() {
		for {
			s.Lock()
			select {
			case <-s.ctx.Done():
				close(s.reschan)
				return
			case s.reschan <- s.scanner.Scan():
			}
		}
	}()
}

func (s *CtxScanner) Scan() bool {
	if s == nil || s.scanner == nil {
		return false
	}
	if s.ctx == nil {
		return s.scanner.Scan()
	}
	s.Do(s.init)

	s.Unlock()
	select {
	case res := <-s.reschan:
		return res
	case <-s.ctx.Done():
		return false
	}
}

func (s *RecoverScanner) Scan() (ret bool) {
	defer func() {
		e := Recover(recover())
		if e != nil {
			ret = false
			s.recovererr = e
		}
	}()
	return s.scanner.Scan()
}

func (s *CtxScanner) Err() error {
	if s.ctx == nil {
		if s.scanner == nil {
			return nil
		}
		return s.scanner.Err()
	}
	select {
	case <-s.ctx.Done():
		return nil
	default:
		return s.scanner.Err()
	}
}

func (s *RecoverScanner) Err() error {
	if s.recovererr != nil {
		return s.recovererr
	}
	return s.scanner.Err()
}

func (s *CtxScanner) Bytes() []byte {
	return s.scanner.Bytes()
}

func (s *CtxScanner) Buffer(buf []byte, max int) {
	s.scanner.Buffer(buf, max)
}

func (s *CtxScanner) Text() string {
	return s.scanner.Text()
}

func (s *CtxScanner) Split(f bufio.SplitFunc) {
	s.scanner.Split(f)
}

func (s *RecoverScanner) Bytes() []byte {
	return s.scanner.Bytes()
}

func (s *RecoverScanner) Buffer(buf []byte, max int) {
	s.scanner.Buffer(buf, max)
}

func (s *RecoverScanner) Text() string {
	return s.scanner.Text()
}

func (s *RecoverScanner) Split(f bufio.SplitFunc) {
	s.scanner.Split(f)
}

func Recover(r interface{}) error {
	e := eerrors.Err(r)
	if e != nil {
		if strings.HasPrefix(e.Error(), "bufio.Scan") {
			return eerrors.WithTypes(eerrors.Wrap(e, "Scanner panicked"), "Panic", "Fatal")
		}
		panic(r)
	}
	return nil
}
