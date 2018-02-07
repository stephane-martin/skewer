package db

import (
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/stephane-martin/skewer/utils"
)

var prefixKeysPool *sync.Pool
var concatPool *sync.Pool

func init() {
	concatPool = &sync.Pool{
		New: func() interface{} { return make([]byte, 0, 32) },
	}
}

func SetBatchSize(size uint32) {
	prefixKeysPool = &sync.Pool{
		New: func() interface{} {
			return make([][]byte, 0, size)
		},
	}
}

func getP() (b []byte) {
	return concatPool.Get().([]byte)[:0]
}

func free(b []byte) {
	concatPool.Put(b)
}

func concatb(prefix []byte, key []byte) (res []byte) {
	res = getP()[:len(prefix)+len(key)]
	copy(res, prefix)
	copy(res[len(prefix):], key[:])
	return res
}

type Transaction interface {
	Commit(callback func(error)) error
	Delete(key []byte) error
	Discard()
	Get(key []byte) (item Item, rerr error)
	NewIterator(opt badger.IteratorOptions) Iterator
	Set(key, val []byte) error
	HasUpdate() bool
}

func View(db *badger.DB, fn func(txn Transaction) error) (err error) {
	txn := NewNTransaction(db, false)
	err = fn(txn)
	txn.Discard()
	return err
}

func PView(db *badger.DB, prefix []byte, fn func(txn Transaction) error) (err error) {
	txn := NewPTransaction(db, prefix, false)
	err = fn(txn)
	txn.Discard()
	return err
}

type Item interface {
	Key() []byte
	Value() ([]byte, error)
	ValueCopy(dst []byte) ([]byte, error)
}

type PItem struct {
	item   Item
	prefix []byte
}

func (i *PItem) Key() []byte {
	return i.item.Key()[len(i.prefix):]
}

func (i *PItem) Value() ([]byte, error) {
	return i.item.Value()
}

func (i *PItem) ValueCopy(dst []byte) ([]byte, error) {
	return i.item.ValueCopy(dst)
}

type Iterator interface {
	Close()
	Item() Item
	Next()
	Rewind()
	Seek(key []byte)
	Valid() bool
	ValidForPrefix(prefix []byte) bool
}

type NIterator struct {
	iterator *badger.Iterator
}

func (i *NIterator) Close() {
	i.iterator.Close()
}

func (i *NIterator) Next() {
	i.iterator.Next()
}

func (i *NIterator) Rewind() {
	i.iterator.Rewind()
}

func (i *NIterator) Seek(key []byte) {
	i.iterator.Seek(key)
}

func (i *NIterator) Valid() bool {
	return i.iterator.Valid()
}

func (i *NIterator) ValidForPrefix(prefix []byte) bool {
	return i.iterator.ValidForPrefix(prefix)
}

func (i *NIterator) Item() Item {
	return i.iterator.Item()
}

type PIterator struct {
	iterator Iterator
	prefix   []byte
}

func (i *PIterator) Close() {
	i.iterator.Close()
}

func (i *PIterator) Next() {
	i.iterator.Next()
}

func (i *PIterator) Rewind() {
	i.iterator.Seek(i.prefix)
}

func (i *PIterator) Seek(key []byte) {
	i.iterator.Seek(concatb(i.prefix, key))
}

func (i *PIterator) Valid() bool {
	return i.iterator.ValidForPrefix(i.prefix)
}

func (i *PIterator) ValidForPrefix(prefix []byte) bool {
	return i.iterator.ValidForPrefix(concatb(i.prefix, prefix))
}

func (i *PIterator) Item() Item {
	return &PItem{item: i.iterator.Item(), prefix: i.prefix}
}

type NTransaction struct {
	txn    *badger.Txn
	update bool
}

func NewNTransaction(db *badger.DB, update bool) Transaction {
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

func (txn *NTransaction) Get(key []byte) (Item, error) {
	return txn.txn.Get(key)
}

func (txn *NTransaction) Set(key, val []byte) error {
	return txn.txn.Set(key, val)
}

func (txn *NTransaction) NewIterator(opt badger.IteratorOptions) Iterator {
	return &NIterator{iterator: txn.txn.NewIterator(opt)}
}

type PTransaction struct {
	txn        Transaction
	prefix     []byte
	prefixKeys [][]byte
	update     bool
}

func NewPTransaction(db *badger.DB, prefix []byte, update bool) Transaction {
	t := PTransaction{
		prefix: prefix,
		txn:    NewNTransaction(db, update),
		update: update,
	}
	if update {
		t.prefixKeys = prefixKeysPool.Get().([][]byte)[:0]
	}
	return &t
}

func (txn *PTransaction) HasUpdate() bool {
	return txn.update
}

func PTransactionFrom(txn Transaction, prefix []byte) Transaction {
	t := PTransaction{prefix: prefix, txn: txn, update: txn.HasUpdate()}
	if t.update {
		t.prefixKeys = prefixKeysPool.Get().([][]byte)[:0]
	}
	return &t
}

func (txn *PTransaction) Commit(callback func(error)) (err error) {
	err = txn.txn.Commit(callback)
	var pkey []byte
	for _, pkey = range txn.prefixKeys {
		free(pkey)
	}
	return err
}

func (txn *PTransaction) Discard() {
	txn.txn.Discard()
	var pkey []byte
	for _, pkey = range txn.prefixKeys {
		free(pkey)
	}
}

func (txn *PTransaction) Delete(key []byte) (err error) {
	pkey := concatb(txn.prefix, key)
	err = txn.txn.Delete(pkey)
	if err == nil {
		txn.prefixKeys = append(txn.prefixKeys, pkey)
	}
	return err
}

func (txn *PTransaction) Get(key []byte) (Item, error) {
	pkey := concatb(txn.prefix, key)
	item, rerr := txn.txn.Get(pkey)
	free(pkey)
	return &PItem{item: item, prefix: txn.prefix}, rerr
}

func (txn *PTransaction) Set(key, val []byte) (err error) {
	pkey := concatb(txn.prefix, key)
	err = txn.txn.Set(pkey, val)
	if err == nil {
		txn.prefixKeys = append(txn.prefixKeys, pkey)
	}
	return err
}

func (txn *PTransaction) NewIterator(opt badger.IteratorOptions) Iterator {
	return &PIterator{iterator: txn.txn.NewIterator(opt), prefix: txn.prefix}
}

type PartitionKeyIterator interface {
	Close()
	Next()
	Rewind()
	Valid() bool
	Key() utils.MyULID
	KeyInto(*utils.MyULID) bool
}

type PartitionKeyValueIterator interface {
	Close()
	Next()
	Rewind()
	Valid() bool
	Key() utils.MyULID
	Value() []byte
	KeyInto(*utils.MyULID) bool
}

type Partition interface {
	ListKeys(txn Transaction) []utils.MyULID
	Count(txn Transaction) int
	Delete(key utils.MyULID, txn Transaction) error
	DeleteMany(keys []utils.MyULID, txn Transaction) error
	Set(key utils.MyULID, value []byte, txn Transaction) error
	AddMany(m map[utils.MyULID][]byte, txn Transaction) error
	AddManySame(uids []utils.MyULID, v []byte, txn Transaction) error
	AddManyTrueMap(m map[utils.MyULID]([]byte), txn Transaction) error
	Get(key utils.MyULID, dst []byte, txn Transaction) ([]byte, error)
	Exists(key utils.MyULID, txn Transaction) (bool, error)
	KeyIterator(prefetchSize uint32, txn Transaction) *ULIDIterator
	KeyValueIterator(prefetchSize uint32, txn Transaction) *ULIDIterator
	View(fn func(Transaction) error) error
}
