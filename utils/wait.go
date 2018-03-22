package utils

import (
	"runtime"
	"time"
)

type ExpWait struct {
	nb uint
}

func (e *ExpWait) Wait() {
	if e.nb < 22 {
		runtime.Gosched()
	} else if e.nb < 24 {
		time.Sleep(time.Millisecond)
	} else if e.nb < 26 {
		time.Sleep(10 * time.Millisecond)
	} else if e.nb < 40 {
		time.Sleep(100 * time.Millisecond)
	} else {
		time.Sleep(250 * time.Millisecond)
	}
	e.nb++
}

func (e *ExpWait) Next() time.Duration {
	e.nb++
	if e.nb < 26 {
		return 10 * time.Millisecond
	}
	if e.nb < 40 {
		return 100 * time.Millisecond
	}
	return 250 * time.Millisecond
}

func (e *ExpWait) Reset() {
	e.nb = 0
}
