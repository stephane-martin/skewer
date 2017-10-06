package queue

import (
	"runtime"
	"sync/atomic"
	"unsafe"
)

type ackNode struct {
	next *ackNode
	msg  string
}

type AckQueue struct {
	head     *ackNode
	tail     *ackNode
	disposed int32
}

func NewAckQueue() *AckQueue {
	stub := &ackNode{}
	q := &AckQueue{head: stub, tail: stub, disposed: 0}
	return q
}

func (q *AckQueue) Disposed() bool {
	return atomic.LoadInt32(&q.disposed) == 1
}

func (q *AckQueue) Dispose() {
	atomic.StoreInt32(&q.disposed, 1)
}

func (q *AckQueue) Put(m string) error {
	n := &ackNode{msg: m}
	if q.Disposed() {
		return ErrDisposed
	}
	(*ackNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *AckQueue) Has() bool {
	return q.tail.next != nil
}

func (q *AckQueue) Wait(stopChan <-chan struct{}) bool {
	for {
		if q.Has() {
			return true
		}
		if q.Disposed() {
			return false
		}
		runtime.Gosched()
	}
}

func WaitManyAckQueues(queues ...*AckQueue) bool {
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
				continue MainLoop
			}
		}
		return false
	}
}

func (q *AckQueue) Get() (s string, err error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		q.tail = next
		tail.msg = next.msg
		s = tail.msg
	} else if q.Disposed() {
		err = ErrDisposed
	}
	return
}

func (q *AckQueue) GetMany(max int) []string {
	var elt string
	var err error
	res := make([]string, 0, max)
	for {
		elt, err = q.Get()
		if (elt == "") || (len(res) > max) || (err != nil) {
			break
		}
		res = append(res, elt)
	}
	return res
}
