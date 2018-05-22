package utils

import (
	"runtime"
	"time"

	"go.uber.org/atomic"
)

type ExpWait struct {
	steps   uint
	maxwait time.Duration
	pos     atomic.Uint64
}

func Waiter(steps uint, maxwait time.Duration) ExpWait {
	if steps <= 0 {
		steps = 5
	}
	if maxwait <= 0 {
		maxwait = 250 * time.Millisecond
	}
	return ExpWait{steps: steps, maxwait: maxwait}
}

// TODO: configurable

func (e *ExpWait) Wait() {
	d := e.Next()
	if d == 0 {
		runtime.Gosched()
		return
	}
	time.Sleep(d)
}

func (e *ExpWait) Next() time.Duration {
	nb := e.pos.Inc() - 1
	if nb < 22 {
		return 0
	} else if nb < 24 {
		return time.Millisecond
	} else if nb < 26 {
		return 10 * time.Millisecond
	} else if nb < 40 {
		return 100 * time.Millisecond
	} else {
		return 250 * time.Millisecond
	}
}

func (e *ExpWait) Reset() {
	e.pos.Store(0)
}
