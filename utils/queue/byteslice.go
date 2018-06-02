package queue

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/waiter"
	uatomic "go.uber.org/atomic"
)

type bsliceNode struct {
	State state
	Next  *bsliceNode
}

type state struct {
	UID utils.MyULID
	S   string
}

type BSliceQueue struct {
	head      *bsliceNode
	_padding1 [8]uint64
	tail      *bsliceNode
	_padding2 [8]uint64
	disposed  uatomic.Bool
}

var bspool = &sync.Pool{New: func() interface{} {
	return new(bsliceNode)
}}

func getBSNode() *bsliceNode {
	return bspool.Get().(*bsliceNode)
}

func NewBSliceQueue() *BSliceQueue {
	stub := new(bsliceNode)
	return &BSliceQueue{head: stub, tail: stub}
}

func (q *BSliceQueue) Disposed() bool {
	return q.disposed.Load()
}

func (q *BSliceQueue) Dispose() {
	q.disposed.Store(true)
}

func (q *BSliceQueue) Has() bool {
	return q.tail.Next != nil
}

func (q *BSliceQueue) Wait(timeout time.Duration) bool {
	start := time.Now()
	w := waiter.Default()
	for {
		if q.Has() {
			return true
		}
		if timeout > 0 && time.Since(start) >= timeout {
			return false
		}
		if q.Disposed() {
			return false
		}
		w.Wait()
	}
}

func (q *BSliceQueue) Get() (bool, utils.MyULID, string, error) {
	tail := q.tail
	next := tail.Next
	if next != nil {
		// prev = q.tail
		// q.tail = q.tail.next
		// prev.slice = next.slice
		(*bsliceNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).State = next.State
		bspool.Put(tail)
		return true, next.State.UID, next.State.S, nil
	}
	if q.Disposed() {
		return false, utils.ZeroULID, "", eerrors.ErrQDisposed
	}
	return false, utils.ZeroULID, "", nil
}

func (q *BSliceQueue) PutSlice(m string) error {
	return q.Put(utils.ZeroULID, m)
}

func (q *BSliceQueue) Put(uid utils.MyULID, m string) error {
	if q.Disposed() {
		return eerrors.ErrQDisposed
	}
	n := getBSNode()
	n.State.S = m
	n.State.UID = uid
	n.Next = nil
	(*bsliceNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).Next = n
	return nil
}

func (q *BSliceQueue) GetMany(max uint32) (slices []string) {
	var (
		slice string
		have  bool
	)
	slices = make([]string, 0, max)
	var i uint32
	for i = 0; i < max; i++ {
		have, _, slice, _ = q.Get()
		if !have {
			break
		}
		slices = append(slices, slice)
	}
	return slices
}

func (q *BSliceQueue) GetManyIntoMap(m *map[utils.MyULID]string, max uint32) {
	var (
		i     uint32
		have  bool
		slice string
		uid   utils.MyULID
	)
	for k := range *m {
		delete(*m, k)
	}
	for i < max {
		have, uid, slice, _ = q.Get()
		if !have {
			break
		}
		(*m)[uid] = slice
		i++
	}
}

func (q *BSliceQueue) GetManyInto(slices *[]string, uids *[]utils.MyULID) {
	var (
		i     int
		have  bool
		slice string
		uid   utils.MyULID
	)

	max := cap(*slices)
	// the previous content of slices and uids is forgotten
	*slices = (*slices)[:0]
	*uids = (*uids)[:0]
	for i < max {
		have, uid, slice, _ = q.Get()
		if !have {
			break
		}
		*slices = append(*slices, slice)
		*uids = append(*uids, uid)
		i++
	}
}

func (q *BSliceQueue) GetManySlicesInto(slices *[]string) {
	var (
		slice string
		have  bool
		i     int
	)
	max := cap(*slices)
	// the previous content of slices is forgotten
	*slices = (*slices)[:0]
	for i < max {
		have, _, slice, _ = q.Get()
		if !have {
			break
		}
		*slices = append(*slices, slice)
		i++
	}
}
