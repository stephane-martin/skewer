package utils

import (
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/stephane-martin/skewer/model"
)

type messsageNode struct {
	next *messsageNode
	msg  *model.TcpUdpParsedMessage
}

type MessageQueue struct {
	head *messsageNode
	tail *messsageNode
}

func NewMessageQueue() *MessageQueue {
	stub := &messsageNode{}
	q := &MessageQueue{head: stub, tail: stub}
	return q
}

func (q *MessageQueue) Put(m model.TcpUdpParsedMessage) {
	n := &messsageNode{msg: &m}
	prev := atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))
	(*messsageNode)(prev).next = n
}

func (q *MessageQueue) Has() bool {
	return q.tail.next != nil
}

func (q *MessageQueue) Wait(stopChan <-chan struct{}) bool {
	for !q.Has() {
		select {
		case <-stopChan:
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
	return true
}

func (q *MessageQueue) Get() *model.TcpUdpParsedMessage {
	tail := q.tail
	next := tail.next
	if next != nil {
		q.tail = next
		tail.msg = next.msg
		return tail.msg
	}
	return nil
}

func (q *MessageQueue) GetMany(max int) []*model.TcpUdpParsedMessage {
	var elt *model.TcpUdpParsedMessage
	res := make([]*model.TcpUdpParsedMessage, 0, max)
	for {
		elt = q.Get()
		if elt == nil || len(res) > max {
			break
		}
		res = append(res, elt)
	}
	return res
}

type ackNode struct {
	next *ackNode
	msg  string
}

type AckQueue struct {
	head *ackNode
	tail *ackNode
}

func NewAckQueue() *AckQueue {
	stub := &ackNode{}
	q := &AckQueue{head: stub, tail: stub}
	return q
}

func (q *AckQueue) Put(m string) {
	n := &ackNode{msg: m}
	prev := atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))
	(*ackNode)(prev).next = n
}

func (q *AckQueue) Has() bool {
	return q.tail.next != nil
}

func (q *AckQueue) Wait(stopChan <-chan struct{}) bool {
	for !q.Has() {
		select {
		case <-stopChan:
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
	return true
}

func WaitManyAckQueues(stopChan <-chan struct{}, queues ...*AckQueue) bool {
	for {
		for _, queue := range queues {
			if queue.Has() {
				return true
			}
		}
		select {
		case <-stopChan:
			return false
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (q *AckQueue) Get() string {
	tail := q.tail
	next := tail.next
	if next != nil {
		q.tail = next
		tail.msg = next.msg
		return tail.msg
	}
	return ""
}

func (q *AckQueue) GetMany(max int) []string {
	var elt string
	res := make([]string, 0, max)
	for {
		elt = q.Get()
		if elt == "" || len(res) > max {
			break
		}
		res = append(res, elt)
	}
	return res
}

/*
void mpscq_push(mpscq_t* self, mpscq_node_t* n)
{
    n->next = 0;
    mpscq_node_t* prev = XCHG(&self->head, n); // serialization-point wrt producers, acquire-release
    prev->next = n; // serialization-point wrt consumer, release
}

mpscq_node_t* mpscq_pop(mpscq_t* self)
{
    mpscq_node_t* tail = self->tail;
    mpscq_node_t* next = tail->next; // serialization-point wrt producers, acquire
    if (next)
    {
        self->tail = next;
        tail->state = next->state;
        return tail;
    }
    return 0;
}
*/
