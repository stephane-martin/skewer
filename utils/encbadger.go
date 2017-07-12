package utils

import "github.com/dgraph-io/badger"

type EncryptedDB struct {
	db     *badger.KV
	secret [32]byte
}

type NonEncryptedDB struct {
	db *badger.KV
}

type DB interface {
	Close() error
	ListKeys() []string
	Delete(key string) error
	DeleteKeys(keys []string) error
	Set(key string, value []byte) error
	Get(key string) ([]byte, error)
}

func NewEncryptedDB(opts *badger.Options, secret [32]byte) (DB, error) {
	kv, err := badger.NewKV(opts)
	if err != nil {
		return nil, err
	}
	return &EncryptedDB{db: kv, secret: secret}, nil
}

func (encDB *EncryptedDB) Close() error {
	return encDB.db.Close()
}

func (encDB *EncryptedDB) ListKeys() []string {
	iter_opts := badger.IteratorOptions{
		PrefetchSize: 1000,
		FetchValues:  false,
		Reverse:      false,
	}

	keys := []string{}
	iter := encDB.db.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		keys = append(keys, string(iter.Item().Key()))
	}
	iter.Close()
	return keys
}

func (encDB *EncryptedDB) Delete(key string) error {
	return encDB.db.Delete([]byte(key))
}

func (encDB *EncryptedDB) DeleteKeys(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	entries := make([]*badger.Entry, 0, len(keys))
	for _, key := range keys {
		entries = badger.EntriesDelete(entries, []byte(key))
	}
	return encDB.db.BatchSet(entries)
}

func (encDB *EncryptedDB) Set(key string, value []byte) error {
	encValue, err := Encrypt(value, encDB.secret)
	if err != nil {
		return err
	}
	return encDB.db.Set([]byte(key), encValue)
}

func (encDB *EncryptedDB) Get(key string) ([]byte, error) {
	enckv := &badger.KVItem{}
	err := encDB.db.Get([]byte(key), enckv)
	if err != nil {
		return nil, err
	}
	decValue, err := Decrypt(enckv.Value(), encDB.secret)
	if err != nil {
		return nil, err
	}
	return decValue, nil
}

func NewNonEncryptedDB(opts *badger.Options) (DB, error) {
	kv, err := badger.NewKV(opts)
	if err != nil {
		return nil, err
	}
	return &NonEncryptedDB{db: kv}, nil
}

func (nDB *NonEncryptedDB) Close() error {
	return nDB.db.Close()
}

func (nDB *NonEncryptedDB) ListKeys() []string {
	iter_opts := badger.IteratorOptions{
		PrefetchSize: 1000,
		FetchValues:  false,
		Reverse:      false,
	}

	keys := []string{}
	iter := nDB.db.NewIterator(iter_opts)
	for iter.Rewind(); iter.Valid(); iter.Next() {
		keys = append(keys, string(iter.Item().Key()))
	}
	iter.Close()
	return keys
}

func (nDB *NonEncryptedDB) Delete(key string) error {
	return nDB.db.Delete([]byte(key))
}

func (nDB *NonEncryptedDB) DeleteKeys(keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	entries := make([]*badger.Entry, 0, len(keys))
	for _, key := range keys {
		entries = badger.EntriesDelete(entries, []byte(key))
	}
	return nDB.db.BatchSet(entries)
}

func (nDB *NonEncryptedDB) Set(key string, value []byte) error {
	return nDB.db.Set([]byte(key), value)
}

func (nDB *NonEncryptedDB) Get(key string) ([]byte, error) {
	nkv := &badger.KVItem{}
	err := nDB.db.Get([]byte(key), nkv)
	if err != nil {
		return nil, err
	}
	return nkv.Value(), nil
}
