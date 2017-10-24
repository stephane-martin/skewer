package db

import "github.com/oklog/ulid"

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
	ListKeys() []ulid.ULID
	Count() int
	Delete(key ulid.ULID) error
	DeleteMany(keys []ulid.ULID) ([]ulid.ULID, error)
	Set(key ulid.ULID, value []byte) error
	AddMany(m map[ulid.ULID][]byte) ([]ulid.ULID, error)
	Get(key ulid.ULID) ([]byte, error)
	Exists(key ulid.ULID) (bool, error)
	KeyIterator(prefetchSize int) PartitionKeyIterator
	KeyValueIterator(prefetchSize int) PartitionKeyValueIterator
}
