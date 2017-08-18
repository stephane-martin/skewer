package utils

import "sync/atomic"
import "github.com/stephane-martin/skewer/model"
import "unsafe"

type node struct {
	next *node
	msg  *model.TcpUdpParsedMessage
}

type MPSC struct {
	head *node
	tail *node
}

func NewMPSC() *MPSC {
	stub := &node{}
	q := &MPSC{}
	q.head = stub
	q.tail = stub
	return q
}

func (q *MPSC) Put(m *model.TcpUdpParsedMessage) {
	n := &node{msg: m}
	prev := atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))
	(*node)(prev).next = n
}

func (q *MPSC) Has() bool {
	return q.tail.next != nil
}

func (q *MPSC) Get() *model.TcpUdpParsedMessage {
	tail := q.tail
	next := tail.next
	if next != nil {
		q.tail = next
		tail.msg = next.msg
		return tail.msg
	}
	return nil
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
