package db

import (
	"github.com/dgraph-io/badger"
	"github.com/oklog/ulid"
)

type partitionImpl struct {
	parent *badger.DB
	prefix []byte
}

func concat(prefix []byte, key ulid.ULID) (res []byte) {
	res = make([]byte, 0, len(prefix)+16)
	res = append(res, prefix...)
	res = append(res, key[:]...)
	return res
}

func (p *partitionImpl) Get(key ulid.ULID, txn *badger.Txn) ([]byte, error) {
	if txn == nil {
		txn = p.parent.NewTransaction(false)
		defer txn.Discard()
	}
	item, err := txn.Get(concat(p.prefix, key))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (p *partitionImpl) Set(key ulid.ULID, value []byte, txn *badger.Txn) (err error) {
	n := false
	if txn == nil {
		txn = p.parent.NewTransaction(true)
		n = true
		defer txn.Discard()
	}
	err = txn.Set(concat(p.prefix, key), value)
	if err != nil {
		txn.Discard()
	} else if n {
		err = txn.Commit(nil)
		if err == badger.ErrConflict {
			// retry
			return p.Set(key, value, nil)
		} else if err != nil {
			return err
		}
	}
	return
}

func (p *partitionImpl) AddMany(m map[ulid.ULID]([]byte), txn *badger.Txn) (err error) {
	if len(m) == 0 {
		return
	}
	n := false
	if txn == nil {
		txn = p.parent.NewTransaction(true)
		n = true
		defer txn.Discard()
	}
	for k, v := range m {
		err = txn.Set(concat(p.prefix, k), v)
		if err != nil {
			txn.Discard()
			return
		}
	}
	if n {
		err = txn.Commit(nil)
		if err == badger.ErrConflict {
			// retry
			return p.AddMany(m, nil)
		} else if err != nil {
			return err
		}
	}
	return
}

func (p *partitionImpl) Exists(key ulid.ULID, txn *badger.Txn) (bool, error) {
	if txn == nil {
		txn = p.parent.NewTransaction(false)
		defer txn.Discard()
	}
	_, err := txn.Get(concat(p.prefix, key))
	if err == nil {
		return true, nil
	} else if err == badger.ErrKeyNotFound {
		return false, nil
	} else {
		return false, err
	}
}

func (p *partitionImpl) Delete(key ulid.ULID, txn *badger.Txn) (err error) {
	n := false
	if txn == nil {
		txn = p.parent.NewTransaction(true)
		n = true
		defer txn.Discard()
	}

	err = txn.Delete(concat(p.prefix, key))
	if err != nil {
		txn.Discard()
	} else if n {
		err = txn.Commit(nil)
		if err == badger.ErrConflict {
			// retry
			return p.Delete(key, nil)
		} else if err != nil {
			return err
		}
	}
	return
}

func (p *partitionImpl) DeleteMany(keys []ulid.ULID, txn *badger.Txn) (err error) {
	if len(keys) == 0 {
		return
	}
	n := false
	if txn == nil {
		txn = p.parent.NewTransaction(true)
		n = true
		defer txn.Discard()
	}

	for _, key := range keys {
		err = txn.Delete(concat(p.prefix, key))
		if err != nil {
			txn.Discard()
			return
		}
	}
	if n {
		err = txn.Commit(nil)
		if err == badger.ErrConflict {
			// retry
			return p.DeleteMany(keys, nil)
		} else if err != nil {
			return err
		}
	}
	return
}

func (p *partitionImpl) ListKeys(txn *badger.Txn) []ulid.ULID {
	if txn == nil {
		txn = p.parent.NewTransaction(false)
		defer txn.Discard()
	}
	l := []ulid.ULID{}
	iter := p.KeyIterator(1000, txn)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		l = append(l, iter.Key())
	}
	iter.Close()
	return l
}

func (p *partitionImpl) Count(txn *badger.Txn) int {
	if txn == nil {
		txn = p.parent.NewTransaction(false)
		defer txn.Discard()
	}
	var l int
	iter := p.KeyIterator(1000, txn)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		l++
	}
	iter.Close()
	return l
}

const MaxUint = ^uint(0)
const MaxInt = int(MaxUint >> 1)

func (p *partitionImpl) KeyIterator(prefetchSize uint32, txn *badger.Txn) PartitionKeyIterator {
	n := false
	if txn == nil {
		txn = p.parent.NewTransaction(false)
		n = true
	}
	var prefetch int
	if uint64(prefetchSize) > uint64(MaxInt) {
		prefetch = MaxInt
	} else {
		prefetch = int(prefetchSize)
	}
	opts := badger.IteratorOptions{
		PrefetchValues: false,
		PrefetchSize:   int(prefetch),
	}
	iter := txn.NewIterator(opts)
	//iter := p.parent.NewIterator(opts)
	return &partitionIterImpl{partition: p, iterator: iter, txn: txn, n: n}
}

func (p *partitionImpl) KeyValueIterator(prefetchSize uint32, txn *badger.Txn) PartitionKeyValueIterator {
	n := false
	if txn == nil {
		txn = p.parent.NewTransaction(false)
		n = true
	}
	var prefetch int
	if uint64(prefetchSize) > uint64(MaxInt) {
		prefetch = MaxInt
	} else {
		prefetch = int(prefetchSize)
	}
	opts := badger.IteratorOptions{
		PrefetchValues: true,
		PrefetchSize:   prefetch,
	}
	iter := txn.NewIterator(opts)
	//iter := p.parent.NewIterator(opts)
	return &partitionIterImpl{partition: p, iterator: iter, txn: txn, n: n}
}

type partitionIterImpl struct {
	partition *partitionImpl
	iterator  *badger.Iterator
	txn       *badger.Txn
	n         bool
}

func (i *partitionIterImpl) Close() {
	i.iterator.Close()
	if i.n {
		i.txn.Discard()
	}
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
	item := i.iterator.Item()
	if item == nil {
		return nil
	} else {
		val, err := item.ValueCopy(nil)
		if err != nil {
			return nil
		}
		return val
	}
}

func NewPartition(parent *badger.DB, prefix []byte) Partition {
	return &partitionImpl{parent: parent, prefix: prefix}
}
