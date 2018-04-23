package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/eerrors"
)

type TopicPartition struct {
	Partition int32
	Topic     string
}

type KafkaProducerAck struct {
	Offset int64
	TopicPartition
}

type kafkaProducerAckNode struct {
	next *kafkaProducerAckNode
	ack  KafkaProducerAck
}

type KafkaProducerAckQueue struct {
	head      *kafkaProducerAckNode
	_padding1 [8]uint64
	tail      *kafkaProducerAckNode
	_padding2 [8]uint64
	disposed  int32
	_padding3 [8]uint64
	pool      *sync.Pool
}

func NewKafkaProducerAckQueue() *KafkaProducerAckQueue {
	stub := &kafkaProducerAckNode{}
	q := &KafkaProducerAckQueue{head: stub, tail: stub, disposed: 0, pool: &sync.Pool{New: func() interface{} {
		return &kafkaProducerAckNode{}
	}}}
	return q
}

func (q *KafkaProducerAckQueue) Disposed() bool {
	return atomic.LoadInt32(&q.disposed) == 1
}

func (q *KafkaProducerAckQueue) Dispose() {
	atomic.StoreInt32(&q.disposed, 1)
}

func (q *KafkaProducerAckQueue) Get() (KafkaProducerAck, error) {
	tail := q.tail
	next := tail.next
	if next != nil {
		(*kafkaProducerAckNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).ack = next.ack
		q.pool.Put(tail)
		return next.ack, nil
	} else if q.Disposed() {
		return KafkaProducerAck{}, eerrors.ErrQDisposed
	}
	return KafkaProducerAck{}, nil
}

func (q *KafkaProducerAckQueue) Put(ack KafkaProducerAck) error {
	n := q.pool.Get().(*kafkaProducerAckNode)
	n.ack = ack
	n.next = nil
	if q.Disposed() {
		return eerrors.ErrQDisposed
	}
	(*kafkaProducerAckNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).next = n
	return nil
}

func (q *KafkaProducerAckQueue) Has() bool {
	return q.tail.next != nil
}

func (q *KafkaProducerAckQueue) Wait() bool {
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

type KafkaQueues struct {
	queues sync.Map
	nextID uint32
}

type WrappedQueue struct {
	*KafkaProducerAckQueue
	qid uint32
}

func (q *WrappedQueue) ID() uint32 {
	return q.qid
}

func NewQueueFactory() *KafkaQueues {
	return &KafkaQueues{}
}

func (qs *KafkaQueues) New() (q *WrappedQueue) {
	q = &WrappedQueue{
		KafkaProducerAckQueue: NewKafkaProducerAckQueue(),
		qid: atomic.AddUint32(&qs.nextID, 1),
	}
	qs.queues.Store(q.qid, q)
	return
}

func (qs *KafkaQueues) Get(qid uint32) *WrappedQueue {
	if q, ok := qs.queues.Load(qid); ok {
		return q.(*WrappedQueue)
	}
	return nil
}

func (qs *KafkaQueues) Delete(q *WrappedQueue) {
	qs.queues.Delete(q.qid)
	q.Dispose()
}
