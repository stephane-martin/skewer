package db

import (
	"github.com/oklog/ulid"
	"github.com/stephane-martin/skewer/utils/sbox"
)

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
	decValue, err := sbox.Decrypt(encVal, i.db.secret)
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
	encValue, err := sbox.Encrypt(value, encDB.secret)
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
		encValue, err = sbox.Encrypt(v, encDB.secret)
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
	decValue, err := sbox.Decrypt(encVal, encDB.secret)
	if err != nil {
		return nil, err
	}
	return decValue, nil
}
