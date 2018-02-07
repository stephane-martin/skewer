package db

import (
	"github.com/dgraph-io/badger"
	"github.com/stephane-martin/skewer/utils"
)

func concat(prefix []byte, key utils.MyULID) (res []byte) {
	res = getP()[:len(prefix)+len(key)]
	copy(res, prefix)
	copy(res[len(prefix):], key[:])
	return res
}

type partitionImpl struct {
	parent *badger.DB
	prefix []byte
}

func (p *partitionImpl) View(fn func(Transaction) error) (err error) {
	txn := NewPTransaction(p.parent, p.prefix, false)
	err = fn(txn)
	txn.Discard()
	return err
}

func (p *partitionImpl) Get(key utils.MyULID, dst []byte, txn Transaction) (ret []byte, err error) {
	txn = PTransactionFrom(txn, p.prefix)
	var item Item
	item, err = txn.Get(key[:])
	if err != nil {
		return nil, err
	}
	if item == nil {
		return dst, nil
	}
	return item.ValueCopy(dst[:0])
}

func (p *partitionImpl) Set(key utils.MyULID, value []byte, txn Transaction) (err error) {
	txn = PTransactionFrom(txn, p.prefix)
	err = txn.Set(key[:], value)
	if err != nil {
		txn.Discard()
	}
	return
}

var trueBytes = []byte("true")

func (p *partitionImpl) AddManyTrueMap(m map[utils.MyULID]([]byte), txn Transaction) (err error) {
	if len(m) == 0 {
		return
	}
	txn = PTransactionFrom(txn, p.prefix)
	var uid utils.MyULID
	for uid = range m {
		err = txn.Set(uid[:], trueBytes)

		if err != nil {
			txn.Discard()
			return
		}
	}
	return
}

func (p *partitionImpl) AddManySame(uids []utils.MyULID, v []byte, txn Transaction) (err error) {
	if len(uids) == 0 {
		return
	}
	txn = PTransactionFrom(txn, p.prefix)
	var uid utils.MyULID
	for _, uid = range uids {
		err = txn.Set(uid[:], v)

		if err != nil {
			txn.Discard()
			return
		}
	}
	return
}

func (p *partitionImpl) AddMany(m map[utils.MyULID]([]byte), txn Transaction) (err error) {
	if len(m) == 0 {
		return
	}
	txn = PTransactionFrom(txn, p.prefix)

	var key utils.MyULID
	var v []byte
	for key, v = range m {
		err = txn.Set(key[:], v)

		if err != nil {
			txn.Discard()
			return
		}
	}
	return
}

func (p *partitionImpl) Exists(key utils.MyULID, txn Transaction) (bool, error) {
	txn = PTransactionFrom(txn, p.prefix)
	_, err := txn.Get(key[:])
	if err == nil {
		return true, nil
	} else if err == badger.ErrKeyNotFound {
		return false, nil
	} else {
		return false, err
	}
}

func (p *partitionImpl) Delete(key utils.MyULID, txn Transaction) (err error) {
	txn = PTransactionFrom(txn, p.prefix)
	err = txn.Delete(key[:])

	if err != nil {
		txn.Discard()
	}
	return
}

func (p *partitionImpl) DeleteMany(keys []utils.MyULID, txn Transaction) (err error) {
	if len(keys) == 0 {
		return
	}
	txn = PTransactionFrom(txn, p.prefix)

	var key utils.MyULID
	for _, key = range keys {
		err = txn.Delete(key[:])
		if err != nil {
			txn.Discard()
			return
		}
	}
	return
}

func (p *partitionImpl) ListKeys(txn Transaction) []utils.MyULID {
	txn = PTransactionFrom(txn, p.prefix)
	l := []utils.MyULID{}
	iter := p.KeyIterator(1000, txn)
	var uid utils.MyULID
	for iter.Rewind(); iter.Valid(); iter.Next() {
		copy(uid[:], iter.Item().Key())
		l = append(l, uid)
	}
	iter.Close()
	return l
}

func (p *partitionImpl) Count(txn Transaction) int {
	txn = PTransactionFrom(txn, p.prefix)
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

func (p *partitionImpl) KeyIterator(prefetchSize uint32, txn Transaction) *ULIDIterator {
	txn = PTransactionFrom(txn, p.prefix)
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
	return &ULIDIterator{Iterator: txn.NewIterator(opts)}
}

func (p *partitionImpl) KeyValueIterator(prefetchSize uint32, txn Transaction) *ULIDIterator {
	txn = PTransactionFrom(txn, p.prefix)
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
	return &ULIDIterator{Iterator: txn.NewIterator(opts)}
}

type ULIDIterator struct {
	Iterator
}

func (i *ULIDIterator) Key() (uid utils.MyULID) {
	copy(uid[:], i.Item().Key())
	return uid
}

func (i *ULIDIterator) KeyInto(uid *utils.MyULID) bool {
	if i == nil || uid == nil {
		return false
	}
	copy((*uid)[:], i.Item().Key())
	return true
}

func (i *ULIDIterator) Value() ([]byte, error) {
	return i.Item().Value()
}

func NewPartition(parent *badger.DB, prefix []byte) Partition {
	return &partitionImpl{parent: parent, prefix: prefix}
}
