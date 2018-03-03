package queue

import (
	//"runtime"

	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/stephane-martin/skewer/model"
	"github.com/stephane-martin/skewer/utils"
)

type messageNode struct {
	next *messageNode
	msg  *model.FullMessage
}

type MessageQueue struct {
	_padding0 [8]uint64
	head      *messageNode
	_padding1 [8]uint64
	tail      *messageNode
	_padding2 [8]uint64
	disposed  int32
	_padding3 [8]uint64
	pool      *sync.Pool
}

func NewMessageQueue() *MessageQueue {
	stub := &messageNode{}
	return &MessageQueue{
		disposed: 0,
		head:     stub,
		tail:     stub,
		pool: &sync.Pool{
			New: func() interface{} {
				return &messageNode{}
			},
		},
	}
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

func (q *MessageQueue) Wait(timeout time.Duration) bool {
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

func (q *MessageQueue) Get() (*model.FullMessage, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		(*messageNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).msg = next.msg
		q.pool.Put(tail)
		return next.msg, nil
	} else if q.Disposed() {
		return nil, utils.ErrDisposed
	} else {
		return nil, nil
	}
}

func (q *MessageQueue) Put(m *model.FullMessage) error {
	if q.Disposed() {
		return utils.ErrDisposed
	}
	n := q.pool.Get().(*messageNode)
	n.msg = m
	n.next = nil
	(*messageNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *MessageQueue) GetMany(max uint32) []*model.FullMessage {
	var elt *model.FullMessage
	var err error
	res := make([]*model.FullMessage, 0, max)
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

func (q *MessageQueue) GetManyInto(msgs *[]*model.FullMessage) {
	var elt *model.FullMessage
	var err error
	var i int
	max := cap(*msgs)
	*msgs = (*msgs)[:0]
	for i < max {
		elt, err = q.Get()
		if elt == nil || err != nil {
			break
		}
		*msgs = append(*msgs, elt)
		i++
	}
}
