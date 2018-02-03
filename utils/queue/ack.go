package queue

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/utils"
)

type ackNode struct {
	next *ackNode
	uid  utils.MyULID
	dest conf.DestinationType
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

func (q *AckQueue) Get() (utils.MyULID, conf.DestinationType, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		(*ackNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).uid = next.uid
		q.pool.Put(tail)
		return next.uid, next.dest, nil
	} else if q.Disposed() {
		return utils.ZeroUid, 0, utils.ErrDisposed
	}
	return utils.ZeroUid, 0, nil
}

func (q *AckQueue) Put(uid utils.MyULID, dest conf.DestinationType) error {
	n := q.pool.Get().(*ackNode)
	n.uid = uid
	n.dest = dest
	n.next = nil
	if q.Disposed() {
		return utils.ErrDisposed
	}
	(*ackNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *AckQueue) Has() bool {
	return q.tail.next != nil
}

func (q *AckQueue) Wait() bool {
	var w utils.ExpWait
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
	var nb uint64
	var q *AckQueue

	notNilQueues := make([]*AckQueue, 0, len(queues))
	for _, q = range queues {
		if q != nil {
			notNilQueues = append(notNilQueues, q)
		}
	}
	if len(notNilQueues) == 0 {
		return false
	}
	queues = notNilQueues

MainLoop:
	for {
		for _, q = range queues {
			if q != nil {
				if q.Has() {
					return true
				}
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

type UidDest struct {
	Uid  utils.MyULID
	Dest conf.DestinationType
}

func (q *AckQueue) GetMany(max uint32) (res []UidDest) {
	var uid utils.MyULID
	var dest conf.DestinationType
	var err error

	res = make([]UidDest, 0, max)
	var i uint32
	for i = 0; i < max; i++ {
		uid, dest, err = q.Get()
		if uid == utils.ZeroUid || err != nil {
			break
		}
		res = append(res, UidDest{Uid: uid, Dest: dest})
	}
	return res
}

func (q *AckQueue) GetUidsExactlyInto(uids *[]utils.MyULID) error {
	var uid utils.MyULID
	var err error
	nb := len(*uids)
	var i int
	for i < nb {
		uid, _, err = q.Get()
		if uid == utils.ZeroUid || err != nil {
			return fmt.Errorf("GetUidsExactlyInto: missing uid for some message")
		}
		(*uids)[i] = uid
		i++
	}
	return nil
}

func (q *AckQueue) GetManyInto(uids *[]UidDest) {
	var uid utils.MyULID
	var dest conf.DestinationType
	var err error
	max := cap(*uids)
	*uids = (*uids)[:0]
	var i int
	for i < max {
		uid, dest, err = q.Get()
		if uid == utils.ZeroUid || err != nil {
			break
		}
		*uids = append(*uids, UidDest{Uid: uid, Dest: dest})
		i++
	}
}
