package queue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/oklog/ulid"
)

var empty ulid.ULID

type ackNode struct {
	next *ackNode
	uid  ulid.ULID
}

type AckQueue struct {
	head     *ackNode
	tail     *ackNode
	disposed int32
	pool     *sync.Pool
}

func NewAckQueue() *AckQueue {
	stub := &ackNode{}
	q := &AckQueue{head: stub, tail: stub, disposed: 0, pool: &sync.Pool{New: func() interface{} {
		return &ackNode{}
	}}}
	return q
}

func (q *AckQueue) Disposed() bool {
	return atomic.LoadInt32(&q.disposed) == 1
}

func (q *AckQueue) Dispose() {
	atomic.StoreInt32(&q.disposed, 1)
}

func (q *AckQueue) Get() (ulid.ULID, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		//q.tail = next
		//tail.msg = next.msg
		//s = tail.msg
		(*ackNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).uid = next.uid
		q.pool.Put(tail)
		return next.uid, nil
	} else if q.Disposed() {
		return empty, ErrDisposed
	}
	return empty, nil
}

func (q *AckQueue) Put(uid ulid.ULID) error {
	n := q.pool.Get().(*ackNode)
	n.uid = uid
	n.next = nil
	if q.Disposed() {
		return ErrDisposed
	}
	(*ackNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *AckQueue) Has() bool {
	return q.tail.next != nil
}

func (q *AckQueue) Wait() bool {
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

func WaitManyAckQueues(queues ...*AckQueue) bool {
	var nb uint64
	var q *AckQueue
MainLoop:
	for {
		for _, q = range queues {
			if q.Has() {
				return true
			}
		}
		for _, q = range queues {
			if !q.Disposed() {
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
				continue MainLoop
			}
		}
		return false
	}
}

func (q *AckQueue) GetMany(max int) []ulid.ULID {
	var elt ulid.ULID
	var err error
	res := make([]ulid.ULID, 0, max)
	for {
		elt, err = q.Get()
		if elt == empty || err != nil {
			break
		}
		res = append(res, elt)
		if len(res) == max {
			break
		}
	}
	return res
}
