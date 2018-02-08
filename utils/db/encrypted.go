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
	p       *partitionImpl
	secret  *memguard.LockedBuffer
	pool    *sync.Pool
	mappool *sync.Pool
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
	return &ULIDIterator{iter: txn.NewIterator(opt), secret: encDB.secret, prefix: encDB.p.prefix}
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
	return &ULIDIterator{iter: txn.NewIterator(opt), secret: encDB.secret, prefix: encDB.p.prefix}
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

func (encDB *EncryptedDB) Set(key utils.MyULID, value []byte, txn *NTransaction) error {
	encValue, err := sbox.Encrypt(value, encDB.secret)
	if err != nil {
		return err
	}
	return encDB.p.Set(key, encValue, txn)
}

func (encDB *EncryptedDB) AddManyTrueMap(m map[utils.MyULID]([]byte), txn *NTransaction) (err error) {
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

func (encDB *EncryptedDB) AddManySame(uids []utils.MyULID, v []byte, txn *NTransaction) (err error) {
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

func (encDB *EncryptedDB) AddMany(m map[utils.MyULID][]byte, txn *NTransaction) error {
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

func (encDB *EncryptedDB) Get(key utils.MyULID, dst []byte, txn *NTransaction) (ret []byte, err error) {
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
