package queue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type intNode struct {
	next *intNode
	uid  int
}

type IntQueue struct {
	head     *intNode
	tail     *intNode
	disposed int32
	pool     *sync.Pool
}

func NewIntQueue() *IntQueue {
	stub := &intNode{}
	q := &IntQueue{head: stub, tail: stub, disposed: 0, pool: &sync.Pool{New: func() interface{} {
		return &intNode{}
	}}}
	return q
}

func (q *IntQueue) Disposed() bool {
	return atomic.LoadInt32(&q.disposed) == 1
}

func (q *IntQueue) Dispose() {
	atomic.StoreInt32(&q.disposed, 1)
}

func (q *IntQueue) Get() (int, error) {
	if q.Disposed() {
		return -1, ErrDisposed
	}
	tail := q.tail
	next := tail.next
	if next != nil {
		(*intNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).uid = next.uid
		q.pool.Put(tail)
		return next.uid, nil
	}
	return -1, nil
}

func (q *IntQueue) Put(uid int) error {
	if q.Disposed() {
		return ErrDisposed
	}
	n := q.pool.Get().(*intNode)
	n.uid = uid
	n.next = nil
	(*intNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *IntQueue) Has() bool {
	return q.tail.next != nil
}

func (q *IntQueue) Wait() bool {
	var nb uint64
	for {
		if q.Has() {
			return true
		}
		if q.Disposed() {
			return false
		}
		if nb < 22 {
			runtime.Gosched()
		} else if nb < 24 {
			time.Sleep(1000000)
		} else if nb < 26 {
			time.Sleep(10000000)
		} else {
			time.Sleep(100000000)
		}
		nb++
	}
}

func WaitOne(q1 *IntQueue, q2 *IntQueue) bool {
	var nb uint64
	for {
		if q1.Disposed() || q2.Disposed() {
			return false
		}
		if q1.Has() || q2.Has() {
			return true
		}
		if nb < 22 {
			runtime.Gosched()
		} else if nb < 24 {
			time.Sleep(time.Millisecond)
		} else if nb < 26 {
			time.Sleep(10 * time.Millisecond)
		} else {
			time.Sleep(100 * time.Millisecond)
		}
		nb++
	}
}
