package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/waiter"
	uatomic "go.uber.org/atomic"
)

type ackNode struct {
	Next  *ackNode
	State aState
}

type aState struct {
	UID  utils.MyULID
	Dest conf.DestinationType
}

var ackPool = &sync.Pool{New: func() interface{} { return &ackNode{} }}

func getACKNode() *ackNode {
	return ackPool.Get().(*ackNode)
}

type AckQueue struct {
	head      *ackNode
	_padding1 [8]uint64
	tail      *ackNode
	_padding2 [8]uint64
	disposed  uatomic.Bool
}

func NewAckQueue() *AckQueue {
	stub := new(ackNode)
	return &AckQueue{head: stub, tail: stub}
}

func (q *AckQueue) Disposed() bool {
	if q == nil {
		return true
	}
	return q.disposed.Load()
}

func (q *AckQueue) Dispose() {
	if q != nil {
		q.disposed.Store(true)
	}
}

func (q *AckQueue) Get() (utils.MyULID, conf.DestinationType, error) {
	if q == nil {
		return utils.ZeroULID, 0, eerrors.ErrQDisposed
	}
	tail := q.tail
	next := tail.Next
	if next != nil {
		(*ackNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).State = next.State
		ackPool.Put(tail)
		return next.State.UID, next.State.Dest, nil
	}
	if q.Disposed() {
		return utils.ZeroULID, 0, eerrors.ErrQDisposed
	}
	return utils.ZeroULID, 0, nil
}

func (q *AckQueue) Put(uid utils.MyULID, dest conf.DestinationType) error {
	if q == nil {
		return eerrors.ErrQDisposed
	}
	n := getACKNode()
	n.State.UID = uid
	n.State.Dest = dest
	n.Next = nil
	if q.Disposed() {
		return eerrors.ErrQDisposed
	}
	(*ackNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).Next = n
	return nil
}

func (q *AckQueue) Has() bool {
	if q == nil {
		return false
	}
	return q.tail.Next != nil
}

func (q *AckQueue) Wait() bool {
	if q == nil {
		return false
	}
	w := waiter.Default()
	for {
		if q.Has() {
			return true
		}
		if q.Disposed() {
			return false
		}
		w.Wait()
	}
}

func WaitManyAckQueues(queues ...*AckQueue) bool {
	var q *AckQueue

	// inplace empty queues filtering
	notNilQueues := queues[:0]
	for _, q = range queues {
		if q != nil {
			notNilQueues = append(notNilQueues, q)
		}
	}
	queues = notNilQueues
	if len(queues) == 0 {
		return false
	}

	w := waiter.Default()

L:
	for {
		for _, q = range queues {
			if q != nil && q.Has() {
				// at least one queue is not empty
				return true
			}
		}
		// all queues are empty
		for _, q = range queues {
			if !q.Disposed() {
				// at least one queue is not disposed
				w.Wait()
				continue L
			}
		}
		// all queues are empty and disposed
		return false
	}
}

type UidDest struct {
	Uid  utils.MyULID
	Dest conf.DestinationType
}

func (q *AckQueue) GetMany(max uint32) (res []UidDest) {
	if q == nil {
		return nil
	}
	var uid utils.MyULID
	var dest conf.DestinationType
	var err error

	res = make([]UidDest, 0, max)
	var i uint32
	for i = 0; i < max; i++ {
		uid, dest, err = q.Get()
		if uid == utils.ZeroULID || err != nil {
			break
		}
		res = append(res, UidDest{Uid: uid, Dest: dest})
	}
	return res
}

func (q *AckQueue) GetManyInto(uids *[]UidDest) {
	if q == nil {
		return
	}
	var uid utils.MyULID
	var dest conf.DestinationType
	var err error
	max := cap(*uids)
	*uids = (*uids)[:0]
	var i int
	for i < max {
		uid, dest, err = q.Get()
		if uid == utils.ZeroULID || err != nil {
			break
		}
		*uids = append(*uids, UidDest{Uid: uid, Dest: dest})
		i++
	}
}
