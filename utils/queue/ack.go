package queue

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/conf"
	"github.com/stephane-martin/skewer/utils"
)

type ackNode struct {
	next *ackNode
	uid  ulid.ULID
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

func (q *AckQueue) Get() (ulid.ULID, conf.DestinationType, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		//q.tail = next
		//tail.msg = next.msg
		//s = tail.msg
		(*ackNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).uid = next.uid
		q.pool.Put(tail)
		return next.uid, next.dest, nil
	} else if q.Disposed() {
		return utils.EmptyUid, 0, utils.ErrDisposed
	}
	return utils.EmptyUid, 0, nil
}

func (q *AckQueue) Put(uid ulid.ULID, dest conf.DestinationType) error {
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

type UidDest struct {
	Uid  ulid.ULID
	Dest conf.DestinationType
}

func (q *AckQueue) GetMany(max int) (res []UidDest) {
	var uid ulid.ULID
	var dest conf.DestinationType
	var err error

	res = make([]UidDest, 0, max)
	for {
		uid, dest, err = q.Get()
		if uid == utils.EmptyUid || err != nil {
			break
		}
		res = append(res, UidDest{Uid: uid, Dest: dest})
		if len(res) == max {
			break
		}
	}
	return res
}
