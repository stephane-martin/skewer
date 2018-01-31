package queue

import (
	"sync/atomic"
	"time"

	"github.com/cheekybits/genny/generic"
	"github.com/stephane-martin/skewer/utils"
)

type Data generic.Type

type node struct {
	position uint64
	data     *Data
}

type nodes []*node

// Ring is a thread-safe bounded queue that stores Data messages.
type Ring struct {
	_padding0      [8]uint64
	queue          uint64
	_padding1      [8]uint64
	dequeue        uint64
	_padding2      [8]uint64
	mask, disposed uint64
	_padding3      [8]uint64
	nodes          nodes
}

func (rb *Ring) init(size uint64) {
	size = utils.RoundUp(size)
	rb.nodes = make(nodes, size)
	for i := uint64(0); i < size; i++ {
		rb.nodes[i] = &node{position: i}
	}
	rb.mask = size - 1 // so we don't have to do this with every put/get operation
}

// Put adds the provided item to the queue.  If the queue is full, this
// call will block until an item is added to the queue or Dispose is called
// on the queue.  An error will be returned if the queue is disposed.
func (rb *Ring) Put(item *Data) error {
	_, err := rb.put(item, false)
	return err
}

// Offer adds the provided item to the queue if there is space.  If the queue
// is full, this call will return false.  An error will be returned if the
// queue is disposed.
func (rb *Ring) Offer(item *Data) (bool, error) {
	return rb.put(item, true)
}

func (rb *Ring) put(item *Data, offer bool) (bool, error) {
	var (
		n *node
		w utils.ExpWait
	)
	pos := atomic.LoadUint64(&rb.queue)
L:
	for {
		if atomic.LoadUint64(&rb.disposed) == 1 {
			return false, utils.ErrDisposed
		}

		n = rb.nodes[pos&rb.mask]
		seq := atomic.LoadUint64(&n.position)
		switch dif := seq - pos; {
		case dif == 0:
			if atomic.CompareAndSwapUint64(&rb.queue, pos, pos+1) {
				break L
			}
		case dif < 0:
			panic(`Ring buffer in a compromised state during a put operation.`)
		default:
			pos = atomic.LoadUint64(&rb.queue)
		}

		if offer {
			return false, nil
		}

		w.Wait()
	}

	n.data = item
	atomic.StoreUint64(&n.position, pos+1)
	return true, nil
}

// Get will return the next item in the queue.  This call will block
// if the queue is empty.  This call will unblock when an item is added
// to the queue or Dispose is called on the queue.  An error will be returned
// if the queue is disposed.
func (rb *Ring) Get() (*Data, error) {
	return rb.Poll(0)
}

func (rb *Ring) PollDeadline(deadline time.Time) (*Data, error) {
	return rb.Poll(deadline.Sub(time.Now()))
}

// Poll will return the next item in the queue.  This call will block
// if the queue is empty.  This call will unblock when an item is added
// to the queue, Dispose is called on the queue, or the timeout is reached. An
// error will be returned if the queue is disposed or a timeout occurs. A
// non-positive timeout will block indefinitely.
func (rb *Ring) Poll(timeout time.Duration) (*Data, error) {
	var (
		n     *node
		pos   = atomic.LoadUint64(&rb.dequeue)
		start time.Time
		w     utils.ExpWait
	)
	if timeout > 0 {
		start = time.Now()
	}
L:
	for {
		n = rb.nodes[pos&rb.mask]
		seq := atomic.LoadUint64(&n.position)
		switch dif := seq - (pos + 1); {
		case dif == 0:
			if atomic.CompareAndSwapUint64(&rb.dequeue, pos, pos+1) {
				break L
			}
		case dif < 0:
			panic(`Ring buffer in compromised state during a get operation.`)
		default:
			pos = atomic.LoadUint64(&rb.dequeue)
		}

		if timeout > 0 && time.Since(start) >= timeout {
			return nil, utils.ErrTimeout
		}
		if atomic.LoadUint64(&rb.disposed) == 1 {
			return nil, utils.ErrDisposed
		}
		w.Wait()
	}
	data := n.data
	n.data = nil
	atomic.StoreUint64(&n.position, pos+rb.mask+1)
	return data, nil
}

// Len returns the number of items in the queue.
func (rb *Ring) Len() uint64 {
	return atomic.LoadUint64(&rb.queue) - atomic.LoadUint64(&rb.dequeue)
}

// Cap returns the capacity of this ring buffer.
func (rb *Ring) Cap() uint64 {
	return uint64(len(rb.nodes))
}

// Dispose will dispose of this queue and free any blocked threads
// in the Put and/or Get methods.  Calling those methods on a disposed
// queue will return an error.
func (rb *Ring) Dispose() {
	atomic.CompareAndSwapUint64(&rb.disposed, 0, 1)
}

// IsDisposed will return a bool indicating if this queue has been
// disposed.
func (rb *Ring) IsDisposed() bool {
	return atomic.LoadUint64(&rb.disposed) == 1
}

// NewRing will allocate, initialize, and return a ring buffer
// with the specified size.
func NewRing(size uint64) *Ring {
	rb := &Ring{}
	rb.init(size)
	return rb
}
