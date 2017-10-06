package queue

import (
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/stephane-martin/skewer/model"
)

type messsageNode struct {
	next *messsageNode
	msg  *model.TcpUdpParsedMessage
}

type MessageQueue struct {
	head     *messsageNode
	tail     *messsageNode
	disposed int32
}

func NewMessageQueue() *MessageQueue {
	stub := &messsageNode{}
	q := &MessageQueue{head: stub, tail: stub, disposed: 0}
	return q
}

func (q *MessageQueue) Disposed() bool {
	return atomic.LoadInt32(&q.disposed) == 1
}

func (q *MessageQueue) Dispose() {
	atomic.StoreInt32(&q.disposed, 1)
}

func (q *MessageQueue) Put(m model.TcpUdpParsedMessage) error {
	n := &messsageNode{msg: &m}
	if q.Disposed() {
		return ErrDisposed
	}
	(*messsageNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *MessageQueue) Has() bool {
	return q.tail.next != nil
}

func (q *MessageQueue) Wait() bool {
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

func (q *MessageQueue) Get() (m *model.TcpUdpParsedMessage, err error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		q.tail = next
		tail.msg = next.msg
		m = tail.msg
	} else if q.Disposed() {
		err = ErrDisposed
	}
	return
}

func (q *MessageQueue) GetMany(max int) []*model.TcpUdpParsedMessage {
	var elt *model.TcpUdpParsedMessage
	var err error
	res := make([]*model.TcpUdpParsedMessage, 0, max)
	for {
		elt, err = q.Get()
		if (elt == nil) || (len(res) > max) || (err != nil) {
			break
		}
		res = append(res, elt)
	}
	return res
}
