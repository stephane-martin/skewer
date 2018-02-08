package db

import (
	"github.com/dgraph-io/badger"
	"github.com/stephane-martin/skewer/utils"
)

type partitionImpl struct {
	parent *badger.DB
	prefix []byte
}

func (p *partitionImpl) Get(key utils.MyULID, dst []byte, txn *NTransaction) (ret []byte, err error) {
	return PTransactionFrom(txn, p.prefix).Get(key[:], dst)
}

func (p *partitionImpl) Set(key utils.MyULID, value []byte, txn *NTransaction) (err error) {
	err = PTransactionFrom(txn, p.prefix).Set(key[:], value)
	if err != nil {
		txn.Discard()
	}
	return
}

var trueBytes = []byte("true")

func (p *partitionImpl) AddManyTrueMap(m map[utils.MyULID]([]byte), txn *NTransaction) (err error) {
	if len(m) == 0 {
		return
	}
	var uid utils.MyULID
	for uid = range m {
		err = PTransactionFrom(txn, p.prefix).Set(uid[:], trueBytes)

		if err != nil {
			txn.Discard()
			return
		}
	}
	return
}

func (p *partitionImpl) AddManySame(uids []utils.MyULID, v []byte, txn *NTransaction) (err error) {
	if len(uids) == 0 {
		return
	}
	ptxn := PTransactionFrom(txn, p.prefix)
	var uid utils.MyULID
	for _, uid = range uids {
		err = ptxn.Set(uid[:], v)

		if err != nil {
			txn.Discard()
			return
		}
	}
	return
}

func (p *partitionImpl) AddMany(m map[utils.MyULID]([]byte), txn *NTransaction) (err error) {
	if len(m) == 0 {
		return
	}
	ptxn := PTransactionFrom(txn, p.prefix)

	var key utils.MyULID
	var v []byte
	for key, v = range m {
		err = ptxn.Set(key[:], v)

		if err != nil {
			txn.Discard()
			return
		}
	}
	return
}

func (p *partitionImpl) Exists(key utils.MyULID, txn *NTransaction) (bool, error) {
	_, err := PTransactionFrom(txn, p.prefix).Get(key[:], nil)
	if err == nil {
		return true, nil
	} else if err == badger.ErrKeyNotFound {
		return false, nil
	} else {
		return false, err
	}
}

func (p *partitionImpl) Delete(key utils.MyULID, txn *NTransaction) (err error) {
	err = PTransactionFrom(txn, p.prefix).Delete(key[:])

	if err != nil {
		txn.Discard()
	}
	return
}

func (p *partitionImpl) DeleteMany(keys []utils.MyULID, txn *NTransaction) (err error) {
	if len(keys) == 0 {
		return
	}
	ptxn := PTransactionFrom(txn, p.prefix)

	var key utils.MyULID
	for _, key = range keys {
		err = ptxn.Delete(key[:])
		if err != nil {
			txn.Discard()
			return
		}
	}
	return
}

func (p *partitionImpl) ListKeys(txn *NTransaction) []utils.MyULID {
	l := []utils.MyULID{}
	iter := p.KeyIterator(1000, txn)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		l = append(l, iter.Key())
	}
	iter.Close()
	return l
}

func (p *partitionImpl) Count(txn *NTransaction) int {
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

func (p *partitionImpl) KeyIterator(prefetchSize uint32, txn *NTransaction) *ULIDIterator {
	var prefetch int
	if uint64(prefetchSize) > uint64(MaxInt) {
		prefetch = MaxInt
	} else {
		prefetch = int(prefetchSize)
	}
	opt := badger.IteratorOptions{
		PrefetchValues: false,
		PrefetchSize:   int(prefetch),
	}
	return &ULIDIterator{iter: txn.NewIterator(opt), secret: nil, prefix: p.prefix}
}

func (p *partitionImpl) KeyValueIterator(prefetchSize uint32, txn *NTransaction) *ULIDIterator {
	var prefetch int
	if uint64(prefetchSize) > uint64(MaxInt) {
		prefetch = MaxInt
	} else {
		prefetch = int(prefetchSize)
	}
	opt := badger.IteratorOptions{
		PrefetchValues: true,
		PrefetchSize:   prefetch,
	}
	return &ULIDIterator{iter: txn.NewIterator(opt), secret: nil, prefix: p.prefix}
}

func NewPartition(parent *badger.DB, prefix []byte) Partition {
	return &partitionImpl{parent: parent, prefix: prefix}
}
