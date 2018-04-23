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
	slice []byte
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

func (q *BSliceQueue) Get() (utils.MyULID, []byte, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		(*bsliceNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).slice = next.slice
		q.pool.Put(tail)
		return next.uid, next.slice, nil
	} else if q.Disposed() {
		return utils.ZeroUid, nil, eerrors.ErrQDisposed
	} else {
		return utils.ZeroUid, nil, nil
	}
}

func (q *BSliceQueue) PutSlice(m []byte) error {
	return q.Put(utils.ZeroUid, m)
}

func (q *BSliceQueue) Put(uid utils.MyULID, m []byte) error {
	// Put puts a *copy* of the m argument into the queue
	if q.Disposed() {
		return eerrors.ErrQDisposed
	}
	n := q.pool.Get().(*bsliceNode)
	if cap(n.slice) >= len(m) {
		// we can reuse the slice
		n.slice = n.slice[:len(m)]
	} else {
		n.slice = make([]byte, len(m), len(m)+36)
	}
	copy(n.slice, m)
	n.uid = uid
	n.next = nil
	(*bsliceNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *BSliceQueue) GetMany(max uint32) (slices [][]byte) {
	var slice []byte
	var err error
	slices = make([][]byte, 0, max)
	var i uint32
	for i = 0; i < max; i++ {
		_, slice, err = q.Get()
		if slice == nil || err != nil {
			break
		}
		slices = append(slices, slice)
	}
	return slices
}

func (q *BSliceQueue) GetManyInto(slices *[]([]byte), uids *[]utils.MyULID) {
	var slice []byte
	var err error
	var i int
	var uid utils.MyULID
	max := cap(*slices)
	// the previous content of slices and uids is forgotten
	*slices = (*slices)[:0]
	*uids = (*uids)[:0]
	for i < max {
		uid, slice, err = q.Get()
		if slice == nil || err != nil {
			break
		}
		*slices = append(*slices, slice)
		*uids = append(*uids, uid)
		i++
	}
}

func (q *BSliceQueue) GetManySlicesInto(slices *[]([]byte)) {
	var slice []byte
	var err error
	var i int
	max := cap(*slices)
	// the previous content of slices is forgotten
	*slices = (*slices)[:0]
	for i < max {
		_, slice, err = q.Get()
		if slice == nil || err != nil {
			break
		}
		*slices = append(*slices, slice)
		i++
	}
}
