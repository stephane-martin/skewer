package utils

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

type EncryptedDB struct {
	db     Partition
	secret [32]byte
}

type encryptedIterator struct {
	iter PartitionKeyValueIterator
	db   *EncryptedDB
}

func (i *encryptedIterator) Close() {
	i.iter.Close()
}

func (i *encryptedIterator) Key() ulid.ULID {
	return i.iter.Key()
}

func (i *encryptedIterator) Next() {
	i.iter.Next()
}

func (i *encryptedIterator) Rewind() {
	i.iter.Rewind()
}

func (i *encryptedIterator) Valid() bool {
	return i.iter.Valid()
}

func (i *encryptedIterator) Value() []byte {
	encVal := i.iter.Value()
	if encVal == nil {
		return nil
	}
	decValue, err := Decrypt(encVal, i.db.secret)
	if err != nil {
		return nil
	}
	return decValue
}

func NewEncryptedPartition(db Partition, secret [32]byte) Partition {
	return &EncryptedDB{db: db, secret: secret}
}

func (encDB *EncryptedDB) KeyIterator(prefetchSize int) PartitionKeyIterator {
	return encDB.db.KeyIterator(prefetchSize)
}

func (encDB *EncryptedDB) KeyValueIterator(prefetchSize int) PartitionKeyValueIterator {
	return &encryptedIterator{iter: encDB.db.KeyValueIterator(prefetchSize), db: encDB}
}

func (encDB *EncryptedDB) Exists(key ulid.ULID) (bool, error) {
	return encDB.db.Exists(key)
}

func (encDB *EncryptedDB) ListKeys() []ulid.ULID {
	return encDB.db.ListKeys()
}

func (encDB *EncryptedDB) Count() int {
	return encDB.db.Count()
}

func (encDB *EncryptedDB) Delete(key ulid.ULID) error {
	return encDB.db.Delete(key)
}

func (encDB *EncryptedDB) DeleteMany(keys []ulid.ULID) ([]ulid.ULID, error) {
	return encDB.db.DeleteMany(keys)
}

func (encDB *EncryptedDB) Set(key ulid.ULID, value []byte) error {
	encValue, err := Encrypt(value, encDB.secret)
	if err != nil {
		return err
	}
	return encDB.db.Set(key, encValue)
}

func (encDB *EncryptedDB) AddMany(m map[ulid.ULID][]byte) (errors []ulid.ULID, err error) {
	var encValue []byte
	errors = []ulid.ULID{}
	encm := map[ulid.ULID][]byte{}

	for k, v := range m {
		encValue, err = Encrypt(v, encDB.secret)
		if err != nil {
			errors = append(errors, k)
		} else {
			encm[k] = encValue
		}
	}
	otherErrors, otherErr := encDB.db.AddMany(encm)
	errors = append(errors, otherErrors...)
	if otherErr != nil {
		err = otherErr
	}
	return
}

func (encDB *EncryptedDB) Get(key ulid.ULID) ([]byte, error) {
	encVal, err := encDB.db.Get(key)
	if err != nil {
		return nil, err
	}
	decValue, err := Decrypt(encVal, encDB.secret)
	if err != nil {
		return nil, err
	}
	return decValue, nil
}
