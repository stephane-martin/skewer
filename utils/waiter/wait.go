package waiter

import (
	"context"
	"runtime"
	"time"

	"go.uber.org/atomic"
)

type W struct {
	steps    uint8
	midsteps uint8
	maxwait  time.Duration
	pos      atomic.Uint64
}

func Default() *W {
	return Waiter(250*time.Millisecond, 50)
}

func Waiter(maxwait time.Duration, steps uint8) (w *W) {
	if steps < 2 {
		steps = 50
	}
	if maxwait <= 0 {
		maxwait = 250 * time.Millisecond
	}
	return &W{steps: steps, maxwait: maxwait, midsteps: steps / 2}
}

func (e *W) Wait() {
	d := e.Next()
	if d == 0 {
		runtime.Gosched()
		return
	}
	time.Sleep(d)
}

func (e *W) WaitCtx(ctx context.Context) {
	if ctx == nil {
		e.Wait()
		return
	}
	d := e.Next()
	if d == 0 {
		runtime.Gosched()
		return
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(d):
		return
	}
}

func pow(x uint64, y uint8) (res uint64) {
	res = 1
	i := uint8(0)
	for i < y {
		res = res * x
		i++
	}
	return res
}

func (e *W) Next() time.Duration {
	nb := e.pos.Inc() - 1
	if nb < uint64(e.midsteps) {
		return 0
	}
	if nb >= uint64(e.steps) {
		return e.maxwait
	}
	d := time.Duration(1000000 * pow(10, uint8(nb-uint64(e.midsteps))))
	if d > e.maxwait {
		return e.maxwait
	}
	return d
}

func (e *W) Reset() {
	e.pos.Store(0)
}
