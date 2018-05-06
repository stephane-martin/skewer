package utils

import (
	"bufio"
	"context"
	"sync"
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
		c:       make(chan bool),
		ctx:     ctx,
	}
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
