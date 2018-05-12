package db

import (
	"github.com/dgraph-io/badger"
	"github.com/stephane-martin/skewer/utils"
)

func concat(prefix, key []byte) []byte {
	var p []byte
	p = append(p, prefix...)
	p = append(p, key...)
	return p
}

type NTransaction struct {
	txn    *badger.Txn
	update bool
}

func NewNTransaction(db *badger.DB, update bool) *NTransaction {
	return &NTransaction{txn: db.NewTransaction(update), update: update}
}

func (txn *NTransaction) HasUpdate() bool {
	return txn.update
}

func (txn *NTransaction) Commit(callback func(error)) error {
	return txn.txn.Commit(callback)
}

func (txn *NTransaction) Discard() {
	txn.txn.Discard()
}

func (txn *NTransaction) Delete(key []byte) (err error) {
	return txn.txn.Delete(key)
}

func (txn *NTransaction) Get(key, dst []byte) ([]byte, error) {
	item, err := txn.txn.Get(key)
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(dst)
}

func (txn *NTransaction) Set(key, val []byte) error {
	return txn.txn.SetWithDiscard(key, val, 0)
}

func (txn *NTransaction) NewIterator(opt badger.IteratorOptions) *badger.Iterator {
	return txn.txn.NewIterator(opt)
}

type PTransaction struct {
	txn    *NTransaction
	prefix []byte
	update bool
}

func NewPTransaction(db *badger.DB, prefix []byte, update bool) *PTransaction {
	return &PTransaction{
		txn:    NewNTransaction(db, update),
		update: update,
		prefix: prefix,
	}
}

func (txn *PTransaction) HasUpdate() bool {
	return txn.update
}

func PTransactionFrom(txn *NTransaction, prefix []byte) *PTransaction {
	return &PTransaction{
		txn:    txn,
		update: txn.HasUpdate(),
		prefix: prefix,
	}
}

func (txn *PTransaction) Commit(callback func(error)) (err error) {
	return txn.txn.Commit(callback)
}

func (txn *PTransaction) Discard() {
	txn.txn.Discard()
}

func (txn *PTransaction) Delete(key []byte) (err error) {
	return txn.txn.Delete(concat(txn.prefix, key))
}

func (txn *PTransaction) Get(key, dst []byte) ([]byte, error) {
	return txn.txn.Get(concat(txn.prefix, key), dst)
}

func (txn *PTransaction) Set(key, val []byte) (err error) {
	return txn.txn.Set(concat(txn.prefix, key), val)
}

type Partition interface {
	ListKeys(txn *NTransaction) []utils.MyULID
	Count(txn *NTransaction) int
	Delete(key utils.MyULID, txn *NTransaction) error
	DeleteMany(keys []utils.MyULID, txn *NTransaction) error
	Set(key utils.MyULID, value []byte, txn *NTransaction) error
	AddMany(m map[utils.MyULID]string, txn *NTransaction) error
	AddManySame(uids []utils.MyULID, v string, txn *NTransaction) error
	AddManyTrueMap(m map[utils.MyULID]string, txn *NTransaction) error
	Get(key utils.MyULID, dst []byte, txn *NTransaction) ([]byte, error)
	Exists(key utils.MyULID, txn *NTransaction) (bool, error)
	KeyIterator(prefetchSize uint32, txn *NTransaction) *ULIDIterator
	KeyValueIterator(prefetchSize uint32, txn *NTransaction) *ULIDIterator
}
