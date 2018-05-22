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

func (i *ULIDIterator) Key() utils.MyULID {
	if len(i.prefix) > 0 {
		return utils.MyULID(string(i.iter.Item().Key()[len(i.prefix):]))
	} else {
		return utils.MyULID(string(i.iter.Item().Key()))
	}
}

func (i *ULIDIterator) KeyInto(uid *utils.MyULID) bool {
	if i == nil || uid == nil {
		return false
	}
	if len(i.prefix) > 0 {
		*uid = utils.MyULID(string(i.iter.Item().Key()[len(i.prefix):]))
	} else {
		*uid = utils.MyULID(string(i.iter.Item().Key()))
	}
	return true
}

func (i *ULIDIterator) Value(dst []byte) ([]byte, error) {
	if i.secret == nil {
		return i.iter.Item().ValueCopy(dst)
	}
	var err error
	var encVal []byte
	var decVal []byte
	// TODO: pool encVal and decVal
	encVal, err = i.iter.Item().ValueCopy(nil)
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

func (i *ULIDIterator) IsDeleted() bool {
	return i.iter.Item().IsDeletedOrExpired()
}
