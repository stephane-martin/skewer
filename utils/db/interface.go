package db

import (
	"github.com/dgraph-io/badger"
	"github.com/stephane-martin/skewer/utils"
)

type PartitionKeyIterator interface {
	Close()
	Next()
	Rewind()
	Valid() bool
	Key() utils.MyULID
}

type PartitionKeyValueIterator interface {
	Close()
	Next()
	Rewind()
	Valid() bool
	Key() utils.MyULID
	Value() []byte
}

type Partition interface {
	ListKeys(txn *badger.Txn) []utils.MyULID
	Count(txn *badger.Txn) int
	Delete(key utils.MyULID, txn *badger.Txn) error
	DeleteMany(keys []utils.MyULID, txn *badger.Txn) error
	Set(key utils.MyULID, value []byte, txn *badger.Txn) error
	AddMany(m map[utils.MyULID][]byte, txn *badger.Txn) error
	Get(key utils.MyULID, txn *badger.Txn) ([]byte, error)
	Exists(key utils.MyULID, txn *badger.Txn) (bool, error)
	KeyIterator(prefetchSize uint32, txn *badger.Txn) PartitionKeyIterator
	KeyValueIterator(prefetchSize uint32, txn *badger.Txn) PartitionKeyValueIterator
}
