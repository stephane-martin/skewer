package utils

import (
	"github.com/stephane-martin/skewer/utils/eerrors"
	"go.uber.org/atomic"
)

type Semaphore struct {
	count    atomic.Int32
	disposed atomic.Bool
	spinlock atomic.Bool
}

func NewSemaphore(total int32) *Semaphore {
	var s Semaphore
	s.count.Store(total)
	return &s
}

func (s *Semaphore) Acquire() error {
	if s.disposed.Load() {
		return eerrors.ErrQDisposed
	}
	var wait ExpWait
	// acquire spinlock
	for !s.spinlock.CAS(false, true) {
		if s.disposed.Load() {
			return eerrors.ErrQDisposed
		}
		wait.Wait()
	}
	defer s.spinlock.Store(false)
	// at most one user of the semaphore arrives here;
	// let's wait for the semaphore to be acquired
	wait.Reset()
	s.count.Dec()
	for s.count.Load() < 0 {
		if s.disposed.Load() {
			s.count.Inc()
			return eerrors.ErrQDisposed
		}
		wait.Wait()
	}
	return nil
}

func (s *Semaphore) Release() {
	s.count.Inc()
}

func (s *Semaphore) Dispose() {
	s.disposed.Store(true)
}
