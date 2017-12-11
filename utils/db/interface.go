package db

import (
	"github.com/dgraph-io/badger"
	"github.com/oklog/ulid"
)

type PartitionKeyIterator interface {
	Close()
	Next()
	Rewind()
	Valid() bool
	Key() ulid.ULID
}

type PartitionKeyValueIterator interface {
	Close()
	Next()
	Rewind()
	Valid() bool
	Key() ulid.ULID
	Value() []byte
}

type Partition interface {
	ListKeys(txn *badger.Txn) []ulid.ULID
	Count(txn *badger.Txn) int
	Delete(key ulid.ULID, txn *badger.Txn) error
	DeleteMany(keys []ulid.ULID, txn *badger.Txn) error
	Set(key ulid.ULID, value []byte, txn *badger.Txn) error
	AddMany(m map[ulid.ULID][]byte, txn *badger.Txn) error
	Get(key ulid.ULID, txn *badger.Txn) ([]byte, error)
	Exists(key ulid.ULID, txn *badger.Txn) (bool, error)
	KeyIterator(prefetchSize uint32, txn *badger.Txn) PartitionKeyIterator
	KeyValueIterator(prefetchSize uint32, txn *badger.Txn) PartitionKeyValueIterator
}
