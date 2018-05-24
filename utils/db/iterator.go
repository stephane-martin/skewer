package db

import (
	"github.com/awnumar/memguard"
	"github.com/dgraph-io/badger"
	"github.com/stephane-martin/skewer/utils"
	"github.com/stephane-martin/skewer/utils/sbox"
)

type ULIDIterator struct {
	iter   *badger.Iterator
	prefix []byte
	secret *memguard.LockedBuffer
}

func (i *ULIDIterator) Close() {
	i.iter.Close()
}

func (i *ULIDIterator) Next() {
	i.iter.Next()
}

func (i *ULIDIterator) Rewind() {
	if len(i.prefix) > 0 {
		i.iter.Seek(i.prefix)
	} else {
		i.iter.Rewind()
	}
}

func (i *ULIDIterator) Valid() bool {
	if len(i.prefix) > 0 {
		return i.iter.ValidForPrefix(i.prefix)
	} else {
		return i.iter.Valid()
	}
}

func (i *ULIDIterator) Key() (uid utils.MyULID) {
	i.KeyInto(&uid)
	return uid
}

func (i *ULIDIterator) KeyInto(uid *utils.MyULID) {
	if len(i.prefix) > 0 {
		*uid = utils.MyULID(string(i.iter.Item().Key()[len(i.prefix):]))
	} else {
		*uid = utils.MyULID(string(i.iter.Item().Key()))
	}
}

func (i *ULIDIterator) Value(dst []byte) ([]byte, error) {
	if i.secret == nil {
		return i.iter.Item().ValueCopy(dst)
	}
	encVal, err := i.iter.Item().ValueCopy(getTmpBuf())
	if err != nil {
		return nil, err
	}
	if len(encVal) == 0 {
		bufpool.Put(encVal)
		return nil, nil
	}
	decVal, err := sbox.DecryptTo(encVal, i.secret, getTmpBuf())
	if err != nil {
		bufpool.Put(encVal)
		return nil, err
	}
	res := append(dst[:0], decVal...)
	bufpool.Put(encVal)
	bufpool.Put(decVal)
	return res, nil
}

func (i *ULIDIterator) IsDeleted() bool {
	return i.iter.Item().IsDeletedOrExpired()
}
