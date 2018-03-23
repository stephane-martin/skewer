package utils

import "sync/atomic"

type Semaphore struct {
	count    int32
	disposed int32
	spinlock int32
}

func NewSemaphore(total int32) *Semaphore {
	return &Semaphore{count: total}
}

func (s *Semaphore) Acquire() error {
	if atomic.LoadInt32(&s.disposed) == 1 {
		return ErrDisposed
	}
	wait := ExpWait{}
	// acquire spinlock
	for !atomic.CompareAndSwapInt32(&s.spinlock, 0, 1) {
		if atomic.LoadInt32(&s.disposed) == 1 {
			return ErrDisposed
		}
		wait.Wait()
	}
	// at most one user of the semaphore arrives here
	// wait for the semaphore to be acquired
	wait = ExpWait{}
	atomic.AddInt32(&s.count, -1)
	for atomic.LoadInt32(&s.count) < 0 {
		if atomic.LoadInt32(&s.disposed) == 1 {
			atomic.AddInt32(&s.count, 1)
			atomic.StoreInt32(&s.spinlock, 0)
			return ErrDisposed
		}
		wait.Wait()
	}
	atomic.StoreInt32(&s.spinlock, 0)
	return nil
}

func (s *Semaphore) Release() {
	atomic.AddInt32(&s.count, 1)
}

func (s *Semaphore) Dispose() {
	atomic.StoreInt32(&s.disposed, 1)
}
