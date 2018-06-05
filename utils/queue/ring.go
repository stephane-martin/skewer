package queue

import (
	"time"

	"github.com/cheekybits/genny/generic"
	"github.com/stephane-martin/skewer/utils/eerrors"
	"github.com/stephane-martin/skewer/utils/waiter"
	"go.uber.org/atomic"
)

type Data generic.Type

type node struct {
	position atomic.Uint64
	data     Data
}

func newNode(pos uint64) *node {
	var n node
	n.position.Store(pos)
	return &n
}

type nodes []*node

// Ring is a thread-safe bounded queue that stores Data messages.
type Ring struct {
	mask      uint64
	_padding0 [8]uint64
	queue     atomic.Uint64
	_padding1 [8]uint64
	dequeue   atomic.Uint64
	_padding2 [8]uint64
	disposed  atomic.Bool
	_padding3 [8]uint64
	nodes     nodes
}

func (rb *Ring) init(size uint64) {
	size = roundUp(size)
	rb.nodes = make(nodes, size)
	for i := uint64(0); i < size; i++ {
		rb.nodes[i] = newNode(i)
	}
	rb.mask = size - 1 // so we don't have to do this with every put/get operation
}

// Put adds the provided item to the queue.  If the queue is full, this
// call will block until an item is added to the queue or Dispose is called
// on the queue.  An error will be returned if the queue is disposed.
func (rb *Ring) Put(item Data) error {
	_, err := rb.put(item, false)
	return err
}

// Offer adds the provided item to the queue if there is space.  If the queue
// is full, this call will return false.  An error will be returned if the
// queue is disposed.
func (rb *Ring) Offer(item Data) (bool, error) {
	return rb.put(item, true)
}

func (rb *Ring) put(item Data, offer bool) (bool, error) {
	var n *node
	w := waiter.Default()
	pos := rb.queue.Load()

	for {
		if rb.disposed.Load() {
			return false, eerrors.ErrQDisposed
		}

		n = rb.nodes[pos&rb.mask]
		seq := n.position.Load()
		if seq == pos {
			if rb.queue.CAS(pos, pos+1) {
				break
			}
		} else {
			pos = rb.queue.Load()
		}

		if offer {
			return false, nil
		}
		w.Wait()
	}

	n.data = item
	n.position.Store(pos + 1)
	return true, nil
}

// Get will return the next item in the queue.  This call will block
// if the queue is empty.  This call will unblock when an item is added
// to the queue or Dispose is called on the queue.  An error will be returned
// if the queue is disposed.
func (rb *Ring) Get() (Data, error) {
	return rb.Poll(0)
}

func (rb *Ring) PollDeadline(deadline time.Time) (Data, error) {
	return rb.Poll(time.Until(deadline))
}

// Poll will return the next item in the queue.  This call will block
// if the queue is empty.  This call will unblock when an item is added
// to the queue, Dispose is called on the queue, or the timeout is reached. An
// error will be returned if the queue is disposed or a timeout occurs. A
// non-positive timeout will block indefinitely.
func (rb *Ring) Poll(timeout time.Duration) (Data, error) {
	var (
		n     *node
		pos   = rb.dequeue.Load()
		start time.Time
		zero  Data
	)
	w := waiter.Default()
	if timeout > 0 {
		start = time.Now()
	}

	for {
		n = rb.nodes[pos&rb.mask]
		seq := n.position.Load()
		if seq == (pos + 1) {
			if rb.dequeue.CAS(pos, pos+1) {
				break
			}
		} else {
			pos = rb.dequeue.Load()
		}

		if rb.disposed.Load() {
			return zero, eerrors.ErrQDisposed
		}
		if timeout < 0 || (timeout > 0 && time.Since(start) >= timeout) {
			return zero, eerrors.ErrQTimeout
		}
		w.Wait()
	}
	data := n.data
	n.data = zero
	n.position.Store(pos + rb.mask + 1)
	return data, nil
}

// Len returns the number of items in the queue.
func (rb *Ring) Len() uint64 {
	if rb == nil {
		return 0
	}
	return rb.queue.Load() - rb.dequeue.Load()
}

// Cap returns the capacity of this ring buffer.
func (rb *Ring) Cap() uint64 {
	if rb == nil {
		return 0
	}
	return uint64(len(rb.nodes))
}

// Dispose will dispose of this queue and free any blocked threads
// in the Put and/or Get methods.  Calling those methods on a disposed
// queue will return an error.
func (rb *Ring) Dispose() {
	if rb != nil {
		rb.disposed.Store(true)
	}
}

// IsDisposed will return a bool indicating if this queue has been
// disposed.
func (rb *Ring) IsDisposed() bool {
	if rb == nil {
		return true
	}
	return rb.disposed.Load()
}

// NewRing will allocate, initialize, and return a ring buffer
// with the specified size.
func NewRing(size uint64) *Ring {
	rb := &Ring{}
	rb.init(size)
	return rb
}

func roundUp(v uint64) uint64 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}
