package utils

import (
	"sync/atomic"

	"github.com/stephane-martin/skewer/utils/ctrie/inttrie"
)

type RefCount struct {
	trie *inttrie.Ctrie
}

func NewRefCount() *RefCount {
	return &RefCount{trie: inttrie.New(nil)}
}

func (r *RefCount) New(uid MyULID, init int32) {
	if init <= 0 {
		return
	}
	r.trie.Insert(string(uid), &init)
}

func (r *RefCount) Dec(uid MyULID) bool {
	ref, have := r.trie.Lookup(string(uid))
	if ref == nil || !have {
		return false
	}
	return atomic.AddInt32(ref, -1) == 0
}

func (r *RefCount) Inc(uid MyULID) {
	ref, have := r.trie.Lookup(string(uid))
	if ref == nil || !have {
		ref = new(int32)
		r.trie.Insert(string(uid), ref)
	}
	atomic.AddInt32(ref, 1)
}

func (r *RefCount) Remove(uid MyULID) {
	r.trie.Remove(string(uid))
}

func (r *RefCount) GC() (ret []MyULID) {
	r.trie.ForEach(func(e inttrie.Entry) {
		if e.Value != nil && *(e.Value) <= 0 {
			ret = append(ret, MyULID(e.Key))
		}
	})
	return ret
}
