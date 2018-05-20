package utils

import (
	"bufio"
	"context"
	"strings"
	"sync"

	"github.com/stephane-martin/skewer/utils/eerrors"
)

type Scanner struct {
	scanner *bufio.Scanner
	c       chan bool
	sync.Mutex
	ctx context.Context
}

func NewWrappedScanner(ctx context.Context, scanner *bufio.Scanner) (s *Scanner) {
	s = &Scanner{
		scanner: scanner,
		ctx:     ctx,
	}
	if ctx == nil {
		return s
	}
	s.c = make(chan bool)
	s.Lock()
	go func() {
		for {
			s.Lock()
			select {
			case <-ctx.Done():
				close(s.c)
				return
			case s.c <- scanner.Scan():
			}
		}
	}()

	return s
}

func (s *Scanner) Scan() bool {
	if s == nil || s.scanner == nil {
		return false
	}
	if s.ctx == nil {
		return s.scanner.Scan()
	}
	s.Unlock()
	select {
	case res := <-s.c:
		return res
	case <-s.ctx.Done():
		return false
	}
}

func (s *Scanner) Err() error {
	return s.scanner.Err()
}

func (s *Scanner) Bytes() []byte {
	return s.scanner.Bytes()
}

func (s *Scanner) Buffer(buf []byte, max int) {
	s.scanner.Buffer(buf, max)
}

func (s *Scanner) Text() string {
	return s.scanner.Text()
}

func (s *Scanner) Split(f bufio.SplitFunc) {
	s.scanner.Split(f)
}

func Recover(r interface{}) error {
	e := eerrors.Err(r)
	if e != nil {
		if strings.HasPrefix(e.Error(), "bufio.Scan") {
			return eerrors.Wrap(e, "Scanner panicked")
		} else {
			panic(r)
		}
	}
	return nil
}

type scanner interface {
	Scan() bool
}

func ScanRecover(s scanner) (ret bool, err error) {
	defer func() {
		err = Recover(recover())
	}()
	ret = s.Scan()
	return
}
