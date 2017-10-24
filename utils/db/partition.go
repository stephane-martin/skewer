package db

import (
	"github.com/dgraph-io/badger"
	"github.com/oklog/ulid"
)

type partitionImpl struct {
	parent *badger.KV
	prefix []byte
}

func (p *partitionImpl) Get(key ulid.ULID) ([]byte, error) {
	val := []byte{}
	item := &badger.KVItem{}
	err := p.parent.Get(append(p.prefix, key[:]...), item)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	err = item.Value(func(v []byte) error {
		val = append(val, v...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (p *partitionImpl) Set(key ulid.ULID, value []byte) error {
	return p.parent.Set(append(p.prefix, key[:]...), value, byte(0))
}

func (p *partitionImpl) AddMany(m map[ulid.ULID][]byte) (errors []ulid.ULID, err error) {
	errors = []ulid.ULID{}
	if len(m) == 0 {
		return
	}
	entries := make([]*badger.Entry, 0, len(m))
	for k, v := range m {
		entries = badger.EntriesSet(entries, append(p.prefix, k[:]...), v)
	}
	err = p.parent.BatchSet(entries)
	if err == nil {
		return
	}
	for _, entry := range entries {
		if entry.Error != nil {
			var uid ulid.ULID
			copy(uid[:], entry.Key[len(p.prefix):])
			errors = append(errors, uid)
		}
	}
	return
}

func (p *partitionImpl) Exists(key ulid.ULID) (bool, error) {
	return p.parent.Exists(append(p.prefix, key[:]...))
}

func (p *partitionImpl) Delete(key ulid.ULID) error {
	return p.parent.Delete(append(p.prefix, key[:]...))
}

func (p *partitionImpl) DeleteMany(keys []ulid.ULID) (errors []ulid.ULID, err error) {
	errors = []ulid.ULID{}
	if len(keys) == 0 {
		return
	}
	entries := make([]*badger.Entry, 0, len(keys))
	for _, key := range keys {
		entries = badger.EntriesDelete(entries, append(p.prefix, key[:]...))
	}
	err = p.parent.BatchSet(entries)
	if err == nil {
		return
	}
	for _, entry := range entries {
		if entry.Error != nil {
			var uid ulid.ULID
			copy(uid[:], entry.Key[len(p.prefix):])
			errors = append(errors, uid)
		}
	}
	return
}

func (p *partitionImpl) ListKeys() []ulid.ULID {
	l := []ulid.ULID{}
	iter := p.KeyIterator(1000)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		l = append(l, iter.Key())
	}
	iter.Close()
	return l
}

func (p *partitionImpl) Count() int {
	var l int
	iter := p.KeyIterator(1000)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		l++
	}
	iter.Close()
	return l
}

func (p *partitionImpl) KeyIterator(prefetchSize int) PartitionKeyIterator {
	opts := badger.IteratorOptions{
		PrefetchValues: false,
		PrefetchSize:   prefetchSize,
		Reverse:        false,
	}
	iter := p.parent.NewIterator(opts)
	return &partitionIterImpl{partition: p, iterator: iter}
}

func (p *partitionImpl) KeyValueIterator(prefetchSize int) PartitionKeyValueIterator {
	opts := badger.IteratorOptions{
		PrefetchValues: true,
		PrefetchSize:   prefetchSize,
		Reverse:        false,
	}
	iter := p.parent.NewIterator(opts)
	return &partitionIterImpl{partition: p, iterator: iter}
}

type partitionIterImpl struct {
	partition *partitionImpl
	iterator  *badger.Iterator
}

func (i *partitionIterImpl) Close() {
	i.iterator.Close()
}

func (i *partitionIterImpl) Rewind() {
	i.iterator.Seek([]byte(i.partition.prefix))
}

func (i *partitionIterImpl) Next() {
	i.iterator.Next()
}

func (i *partitionIterImpl) Valid() bool {
	return i.iterator.ValidForPrefix([]byte(i.partition.prefix))
}

func (i *partitionIterImpl) Key() (uid ulid.ULID) {
	item := i.iterator.Item()
	if item == nil {
		return uid
	} else {
		key := item.Key()
		if key == nil {
			return uid
		} else {
			copy(uid[:], key[len(i.partition.prefix):])
			return uid
		}
	}
}

func (i *partitionIterImpl) Value() []byte {
	val := []byte{}
	item := i.iterator.Item()
	if item == nil {
		return nil
	} else {
		item.Value(func(v []byte) error {
			val = append(val, v...)
			return nil
		})
		return val
	}
}

func NewPartition(parent *badger.KV, prefix []byte) Partition {
	return &partitionImpl{parent: parent, prefix: prefix}
}
