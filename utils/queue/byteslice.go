package queue

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

type bsliceNode struct {
	uid   utils.MyULID
	next  *bsliceNode
	slice string
}

type BSliceQueue struct {
	head      *bsliceNode
	_padding1 [8]uint64
	tail      *bsliceNode
	_padding2 [8]uint64
	disposed  int32
	_padding3 [8]uint64
	pool      *sync.Pool
}

func NewBSliceQueue() *BSliceQueue {
	stub := new(bsliceNode)
	return &BSliceQueue{
		head: stub,
		tail: stub,
		pool: &sync.Pool{
			New: func() interface{} {
				return new(bsliceNode)
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
		if q.Has() {
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

func (q *BSliceQueue) Get() (utils.MyULID, string, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		// prev = q.tail
		// q.tail = q.tail.next
		// prev.slice = next.slice
		(*bsliceNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).slice = next.slice
		q.pool.Put(tail)
		return next.uid, string(next.slice), nil
	} else if q.Disposed() {
		return utils.ZeroULID, "", eerrors.ErrQDisposed
	} else {
		return utils.ZeroULID, "", nil
	}
}

func (q *BSliceQueue) PutSlice(m string) error {
	return q.Put(utils.ZeroULID, m)
}

func (q *BSliceQueue) Put(uid utils.MyULID, m string) error {
	// Put puts a *copy* of the m argument into the queue
	if q.Disposed() {
		return eerrors.ErrQDisposed
	}
	n := q.pool.Get().(*bsliceNode)
	n.slice = m
	n.uid = uid
	n.next = nil
	// prev = q.head
	// q.head = n
	// prev.next = n
	(*bsliceNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *BSliceQueue) GetMany(max uint32) (slices []string) {
	var slice string
	var err error
	slices = make([]string, 0, max)
	var i uint32
	for i = 0; i < max; i++ {
		_, slice, err = q.Get()
		if slice == "" || err != nil {
			break
		}
		slices = append(slices, slice)
	}
	return slices
}

func (q *BSliceQueue) GetManyInto(slices *[]string, uids *[]utils.MyULID) {
	var slice string
	var err error
	var i int
	var uid utils.MyULID
	max := cap(*slices)
	// the previous content of slices and uids is forgotten
	*slices = (*slices)[:0]
	*uids = (*uids)[:0]
	for i < max {
		uid, slice, err = q.Get()
		if slice == "" || err != nil {
			break
		}
		*slices = append(*slices, slice)
		*uids = append(*uids, uid)
		i++
	}
}

func (q *BSliceQueue) GetManySlicesInto(slices *[]string) {
	var slice string
	var err error
	var i int
	max := cap(*slices)
	// the previous content of slices is forgotten
	*slices = (*slices)[:0]
	for i < max {
		_, slice, err = q.Get()
		if slice == "" || err != nil {
			break
		}
		*slices = append(*slices, slice)
		i++
	}
}
