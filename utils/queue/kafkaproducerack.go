package queue

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/waiter"
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
	Next  *kafkaProducerAckNode
	State KafkaProducerAck
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

var zeroAck KafkaProducerAck

func NewKafkaProducerAckQueue() *KafkaProducerAckQueue {
	stub := new(kafkaProducerAckNode)
	q := &KafkaProducerAckQueue{
		head:     stub,
		tail:     stub,
		disposed: 0,
		pool: &sync.Pool{
			New: func() interface{} {
				return new(kafkaProducerAckNode)
			},
		},
	}
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
	next := tail.Next
	if next != nil {
		(*kafkaProducerAckNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.tail)), unsafe.Pointer(next))).State = next.State
		q.pool.Put(tail)
		return next.State, nil
	} else if q.Disposed() {
		return zeroAck, eerrors.ErrQDisposed
	}
	return zeroAck, nil
}

func (q *KafkaProducerAckQueue) Put(offset int64, partition int32, topic string) error {
	ack := KafkaProducerAck{
		Offset: offset,
		TopicPartition: TopicPartition{
			Partition: partition,
			Topic:     topic,
		},
	}
	n := q.pool.Get().(*kafkaProducerAckNode)
	n.State = ack
	n.Next = nil
	if q.Disposed() {
		return eerrors.ErrQDisposed
	}
	(*kafkaProducerAckNode)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n))).Next = n
	return nil
}

func (q *KafkaProducerAckQueue) Has() bool {
	return q.tail.Next != nil
}

func (q *KafkaProducerAckQueue) Wait() bool {
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
