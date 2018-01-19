package queue

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/stephane-martin/skewer/utils"
)

type bsliceNode struct {
	next  *bsliceNode
	slice []byte
}

type BSliceQueue struct {
	head     *bsliceNode
	tail     *bsliceNode
	disposed int32
	pool     *sync.Pool
}

func NewBSliceQueue() *BSliceQueue {
	stub := &bsliceNode{}
	return &BSliceQueue{
		disposed: 0,
		head:     stub,
		tail:     stub,
		pool: &sync.Pool{
			New: func() interface{} {
				return &bsliceNode{}
			},
		},
	}
}

func (q *BSliceQueue) Disposed() bool {
	return atomic.LoadInt32(&q.disposed) == 1
}

func (q *BSliceQueue) Dispose() {
	atomic.StoreInt32(&q.disposed, 1)
}

func (q *BSliceQueue) Has() bool {
	return q.tail.next != nil
}

func (q *BSliceQueue) Wait(timeout time.Duration) bool {
	start := time.Now()
	var w utils.ExpWait
	for {
		if q.tail.next != nil {
			return true
		}
		if timeout > 0 && time.Since(start) >= timeout {
			return false
		}
		if atomic.LoadInt32(&q.disposed) == 1 {
			return false
		}
		w.Wait()
	}
}

func (q *BSliceQueue) Get() ([]byte, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		//q.tail = next
		//tail.msg = next.msg
		//m = tail.msg
		(*bsliceNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).slice = next.slice
		q.pool.Put(tail)
		return next.slice, nil
	} else if q.Disposed() {
		return nil, utils.ErrDisposed
	} else {
		return nil, nil
	}
}

func (q *BSliceQueue) Put(m []byte) error {
	if q.Disposed() {
		return utils.ErrDisposed
	}
	n := q.pool.Get().(*bsliceNode)
	n.slice = m
	n.next = nil
	(*bsliceNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *BSliceQueue) GetMany(max uint32) [][]byte {
	var elt []byte
	var err error
	res := make([][]byte, 0, max)
	var i uint32
	for i = 0; i < max; i++ {
		elt, err = q.Get()
		if elt == nil || err != nil {
			break
		}
		res = append(res, elt)
	}
	return res
}
