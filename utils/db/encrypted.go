package db

import (
	"sync"

	"github.com/awnumar/memguard"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/sbox"
)

type EncryptedDB struct {
	p       Partition
	secret  *memguard.LockedBuffer
	pool    *sync.Pool
	mappool *sync.Pool
}

type encryptedItem struct {
	item   Item
	secret *memguard.LockedBuffer
}

func (i *encryptedItem) Key() []byte {
	return i.item.Key()
}

func (i *encryptedItem) Value() ([]byte, error) {
	var err error
	var encVal []byte
	var decVal []byte
	encVal, err = i.item.Value()
	if err != nil {
		return nil, err
	}
	if encVal == nil {
		return nil, nil
	}
	decVal, err = sbox.Decrypt(encVal, i.secret)
	if err != nil {
		return nil, err
	}
	return decVal, nil
}

func (i *encryptedItem) ValueCopy(dst []byte) ([]byte, error) {
	var err error
	var encVal []byte
	var decVal []byte
	encVal, err = i.item.Value()
	if err != nil {
		return nil, err
	}
	if encVal == nil {
		return nil, nil
	}
	decVal, err = sbox.Decrypt(encVal, i.secret)
	if err != nil {
		return nil, err
	}
	return append(dst[:0], decVal...), nil
}

type encryptedIterator struct {
	iter Iterator
	p    *EncryptedDB
}

func (i *encryptedIterator) Close() {
	i.iter.Close()
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

func (i *encryptedIterator) ValidForPrefix(prefix []byte) bool {
	return i.iter.ValidForPrefix(prefix)
}

func (i *encryptedIterator) Seek(key []byte) {
	i.iter.Seek(key)
}

func (i *encryptedIterator) Item() Item {
	return &encryptedItem{item: i.iter.Item(), secret: i.p.secret}
}

func NewEncryptedPartition(p Partition, secret *memguard.LockedBuffer) Partition {
	return &EncryptedDB{
		p:      p,
		secret: secret,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 4096)
			},
		},
		mappool: &sync.Pool{
			New: func() interface{} {
				return make(map[utils.MyULID][]byte, 4096)
			},
		},
	}
}

func (encDB *EncryptedDB) View(fn func(Transaction) error) error {
	return encDB.p.View(fn)
}

func (encDB *EncryptedDB) KeyIterator(prefetchSize uint32, txn Transaction) *ULIDIterator {
	return encDB.p.KeyIterator(prefetchSize, txn)
}

func (encDB *EncryptedDB) KeyValueIterator(prefetchSize uint32, txn Transaction) *ULIDIterator {
	return &ULIDIterator{Iterator: &encryptedIterator{iter: encDB.p.KeyValueIterator(prefetchSize, txn), p: encDB}}
}

func (encDB *EncryptedDB) Exists(key utils.MyULID, txn Transaction) (bool, error) {
	return encDB.p.Exists(key, txn)
}

func (encDB *EncryptedDB) ListKeys(txn Transaction) []utils.MyULID {
	return encDB.p.ListKeys(txn)
}

func (encDB *EncryptedDB) Count(txn Transaction) int {
	return encDB.p.Count(txn)
}

func (encDB *EncryptedDB) Delete(key utils.MyULID, txn Transaction) error {
	return encDB.p.Delete(key, txn)
}

func (encDB *EncryptedDB) DeleteMany(keys []utils.MyULID, txn Transaction) error {
	return encDB.p.DeleteMany(keys, txn)
}

func (encDB *EncryptedDB) Set(key utils.MyULID, value []byte, txn Transaction) error {
	encValue, err := sbox.Encrypt(value, encDB.secret)
	if err != nil {
		return err
	}
	return encDB.p.Set(key, encValue, txn)
}

func (encDB *EncryptedDB) AddManyTrueMap(m map[utils.MyULID]([]byte), txn Transaction) (err error) {
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

func (encDB *EncryptedDB) AddManySame(uids []utils.MyULID, v []byte, txn Transaction) (err error) {
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

func (encDB *EncryptedDB) AddMany(m map[utils.MyULID][]byte, txn Transaction) error {
	var err error
	var uid utils.MyULID
	var val, encVal []byte
	tmpMap := encDB.mappool.Get().(map[utils.MyULID][]byte)
	// first clean the tmpMap
	for uid = range tmpMap {
		delete(tmpMap, uid)
	}

	for uid, val = range m {
		encVal, err = sbox.Encrypt(val, encDB.secret)
		if err != nil {
			return err
		}
		tmpMap[uid] = encVal
	}
	err = encDB.p.AddMany(tmpMap, txn)
	encDB.mappool.Put(tmpMap)
	return err
}

func (encDB *EncryptedDB) Get(key utils.MyULID, dst []byte, txn Transaction) (ret []byte, err error) {
	encValBuf := encDB.pool.Get().([]byte)
	encVal := encValBuf[:0]
	encVal, err = encDB.p.Get(key, encVal, txn)
	if err != nil {
		return dst, err
	}
	ret, err = sbox.DecrypTo(encVal, encDB.secret, dst[:0])
	encDB.pool.Put(encValBuf)
	return ret, err
}
