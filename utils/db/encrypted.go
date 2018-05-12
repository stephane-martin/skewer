package db

import (
	"fmt"
	"sync"

	"github.com/awnumar/memguard"
	"github.com/dgraph-io/badger"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/sbox"
)

type EncryptedDB struct {
	p      *partitionImpl
	secret *memguard.LockedBuffer
}

var bufpool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 4096)
	},
}

var mappool = &sync.Pool{
	New: func() interface{} {
		return make(map[utils.MyULID]string, 5000)
	},
}

func getTmpMap() map[utils.MyULID]string {
	m := mappool.Get().(map[utils.MyULID]string)
	for k := range m {
		delete(m, k)
	}
	return m
}

func getTmpBuf() []byte {
	return bufpool.Get().([]byte)[:0]
}

func NewEncryptedPartition(p Partition, secret *memguard.LockedBuffer) (Partition, error) {
	var impl *partitionImpl
	var ok bool
	if impl, ok = p.(*partitionImpl); !ok {
		return nil, fmt.Errorf("Argument partition is not a partitionImpl")
	}
	return &EncryptedDB{
		p:      impl,
		secret: secret,
	}, nil
}

func (encDB *EncryptedDB) KeyIterator(prefetchSize uint32, txn *NTransaction) *ULIDIterator {
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
	return &ULIDIterator{iter: txn.NewIterator(opt), secret: encDB.secret, prefix: []byte(encDB.p.prefix)}
}

func (encDB *EncryptedDB) KeyValueIterator(prefetchSize uint32, txn *NTransaction) *ULIDIterator {
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
	return &ULIDIterator{iter: txn.NewIterator(opt), secret: encDB.secret, prefix: []byte(encDB.p.prefix)}
}

func (encDB *EncryptedDB) Exists(key utils.MyULID, txn *NTransaction) (bool, error) {
	return encDB.p.Exists(key, txn)
}

func (encDB *EncryptedDB) ListKeys(txn *NTransaction) []utils.MyULID {
	return encDB.p.ListKeys(txn)
}

func (encDB *EncryptedDB) Count(txn *NTransaction) int {
	return encDB.p.Count(txn)
}

func (encDB *EncryptedDB) Delete(key utils.MyULID, txn *NTransaction) error {
	return encDB.p.Delete(key, txn)
}

func (encDB *EncryptedDB) DeleteMany(keys []utils.MyULID, txn *NTransaction) error {
	return encDB.p.DeleteMany(keys, txn)
}

func (encDB *EncryptedDB) Set(key utils.MyULID, value string, txn *NTransaction) error {
	encBuf, err := sbox.EncryptTo([]byte(value), encDB.secret, getTmpBuf())
	if err != nil {
		return err
	}
	// we can reuse encBuf because we pass a *copy*
	err = encDB.p.Set(key, string(encBuf), txn)
	bufpool.Put(encBuf)
	return err
}

func (encDB *EncryptedDB) AddManyTrueMap(m map[utils.MyULID]string, txn *NTransaction) (err error) {
	encValue, err := sbox.Encrypt(trueBytes, encDB.secret)
	if err != nil {
		return err
	}
	encStr := string(encValue)

	tmpMap := getTmpMap()
	for uid := range m {
		tmpMap[uid] = encStr
	}
	err = encDB.p.AddMany(tmpMap, txn)
	mappool.Put(tmpMap)
	return err
}

func (encDB *EncryptedDB) AddManySame(uids []utils.MyULID, v string, txn *NTransaction) (err error) {
	encValue, err := sbox.Encrypt([]byte(v), encDB.secret)
	if err != nil {
		return err
	}
	return encDB.p.AddManySame(uids, string(encValue), txn)
}

func (encDB *EncryptedDB) AddMany(m map[utils.MyULID]string, txn *NTransaction) error {
	tmpMap := getTmpMap()

	for uid, val := range m {
		encVal, err := sbox.Encrypt([]byte(val), encDB.secret)
		if err != nil {
			return err
		}
		tmpMap[uid] = string(encVal)
	}
	err := encDB.p.AddMany(tmpMap, txn)
	mappool.Put(tmpMap)
	return err
}

func (encDB *EncryptedDB) Get(key utils.MyULID, dst []byte, txn *NTransaction) ([]byte, error) {
	encVal, err := encDB.p.Get(key, getTmpBuf(), txn)
	if err != nil {
		return dst, err
	}
	ret, err := sbox.DecrypTo(encVal, encDB.secret, dst[:0])
	bufpool.Put(encVal)
	return ret, err
}
