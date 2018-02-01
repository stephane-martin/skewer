package db

import (
	"github.com/awnumar/memguard"
	"github.com/dgraph-io/badger"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/sbox"
)

type EncryptedDB struct {
	p      Partition
	secret *memguard.LockedBuffer
}

type encryptedIterator struct {
	iter PartitionKeyValueIterator
	p    *EncryptedDB
}

func (i *encryptedIterator) Close() {
	i.iter.Close()
}

func (i *encryptedIterator) Key() utils.MyULID {
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
	decValue, err := sbox.Decrypt(encVal, i.p.secret)
	if err != nil {
		return nil
	}
	return decValue
}

func NewEncryptedPartition(p Partition, secret *memguard.LockedBuffer) Partition {
	return &EncryptedDB{p: p, secret: secret}
}

func (encDB *EncryptedDB) KeyIterator(prefetchSize uint32, txn *badger.Txn) PartitionKeyIterator {
	return encDB.p.KeyIterator(prefetchSize, txn)
}

func (encDB *EncryptedDB) KeyValueIterator(prefetchSize uint32, txn *badger.Txn) PartitionKeyValueIterator {
	return &encryptedIterator{iter: encDB.p.KeyValueIterator(prefetchSize, txn), p: encDB}
}

func (encDB *EncryptedDB) Exists(key utils.MyULID, txn *badger.Txn) (bool, error) {
	return encDB.p.Exists(key, txn)
}

func (encDB *EncryptedDB) ListKeys(txn *badger.Txn) []utils.MyULID {
	return encDB.p.ListKeys(txn)
}

func (encDB *EncryptedDB) Count(txn *badger.Txn) int {
	return encDB.p.Count(txn)
}

func (encDB *EncryptedDB) Delete(key utils.MyULID, txn *badger.Txn) error {
	return encDB.p.Delete(key, txn)
}

func (encDB *EncryptedDB) DeleteMany(keys []utils.MyULID, txn *badger.Txn) error {
	return encDB.p.DeleteMany(keys, txn)
}

func (encDB *EncryptedDB) Set(key utils.MyULID, value []byte, txn *badger.Txn) error {
	encValue, err := sbox.Encrypt(value, encDB.secret)
	if err != nil {
		return err
	}
	return encDB.p.Set(key, encValue, txn)
}

func (encDB *EncryptedDB) AddManyTrueMap(m map[utils.MyULID]([]byte), txn *badger.Txn) (err error) {
	encm := make(map[utils.MyULID]([]byte), len(m))
	encValue, err := sbox.Encrypt(trueBytes, encDB.secret)
	if err != nil {
		return err
	}
	for uid := range m {
		encm[uid] = encValue
	}
	return encDB.p.AddMany(encm, txn)
}

func (encDB *EncryptedDB) AddManySame(uids []utils.MyULID, v []byte, txn *badger.Txn) (err error) {
	encm := make(map[utils.MyULID]([]byte), len(uids))
	encValue, err := sbox.Encrypt(v, encDB.secret)
	if err != nil {
		return err
	}
	for _, uid := range uids {
		encm[uid] = encValue
	}
	return encDB.p.AddMany(encm, txn)
}

func (encDB *EncryptedDB) AddMany(m map[utils.MyULID][]byte, txn *badger.Txn) (err error) {
	var encValue []byte
	encm := map[utils.MyULID][]byte{}

	for k, v := range m {
		encValue, err = sbox.Encrypt(v, encDB.secret)
		if err != nil {
			return err
		}
		encm[k] = encValue
	}
	return encDB.p.AddMany(encm, txn)
}

func (encDB *EncryptedDB) Get(key utils.MyULID, txn *badger.Txn) ([]byte, error) {
	encVal, err := encDB.p.Get(key, txn)
	if err != nil {
		return nil, err
	}
	decValue, err := sbox.Decrypt(encVal, encDB.secret)
	if err != nil {
		return nil, err
	}
	return decValue, nil
}
