package queue

import (
	//"runtime"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/stephane-martin/skewer/model"
)

type messageNode struct {
	next *messageNode
	msg  *model.TcpUdpParsedMessage
}

type MessageQueue struct {
	head     *messageNode
	tail     *messageNode
	disposed int32
	pool     *sync.Pool
}

func NewMessageQueue() *MessageQueue {
	stub := &messageNode{}
	return &MessageQueue{head: stub, tail: stub, disposed: 0, pool: &sync.Pool{New: func() interface{} {
		return &messageNode{}
	}}}
}

func (q *MessageQueue) Disposed() bool {
	return atomic.LoadInt32(&q.disposed) == 1
}

func (q *MessageQueue) Dispose() {
	atomic.StoreInt32(&q.disposed, 1)
}

func (q *MessageQueue) Has() bool {
	return q.tail.next != nil
}

func (q *MessageQueue) Wait() bool {
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
			time.Sleep(time.Millisecond)
		} else if nb < 26 {
			time.Sleep(10 * time.Millisecond)
		} else {
			time.Sleep(100 * time.Millisecond)
		}
		nb++
	}
}

func (q *MessageQueue) Get() (*model.TcpUdpParsedMessage, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		//q.tail = next
		//tail.msg = next.msg
		//m = tail.msg
		(*messageNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).msg = next.msg
		q.pool.Put(tail)
		return next.msg, nil
	} else if q.Disposed() {
		return nil, ErrDisposed
	} else {
		return nil, nil
	}
}

func (q *MessageQueue) Put(m model.TcpUdpParsedMessage) error {
	if q.Disposed() {
		return ErrDisposed
	}
	n := q.pool.Get().(*messageNode)
	n.msg = &m
	n.next = nil
	(*messageNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	// q.head.next = n
	// q.head = n
	return nil
}

func (q *MessageQueue) GetMany(max int) []*model.TcpUdpParsedMessage {
	var elt *model.TcpUdpParsedMessage
	var err error
	res := make([]*model.TcpUdpParsedMessage, 0, max)
	for {
		elt, err = q.Get()
		if elt == nil || err != nil {
			break
		}
		res = append(res, elt)
		if len(res) == max {
			break
		}
	}
	return res
}
