// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"sync"

	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
)

// badgerDB is KVStore implementation based bolt DB
type badgerDB struct {
	mutex  sync.RWMutex
	db     *badger.DB
	path   string
	config config.DB
}

// NewBadgerDB instantiates a badgerDB based KV store
func NewBadgerDB(path string, cfg config.DB) KVStore {
	return &badgerDB{db: nil, path: path, config: cfg}
}

// Start opens the badgerDB (creates new file if not existing yet)
func (b *badgerDB) Start(_ context.Context) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.db != nil {
		return nil
	}

	opts := badger.DefaultOptions
	opts.Dir = b.path
	opts.ValueDir = b.path
	db, err := badger.Open(opts)
	if err != nil {
		return err
	}
	b.db = db
	return nil
}

// Stop closes the badgerDB
func (b *badgerDB) Stop(_ context.Context) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.db != nil {
		err := b.db.Close()
		b.db = nil
		return err
	}
	return nil
}

// Put inserts a <key, value> record
func (b *badgerDB) Put(namespace string, key, value []byte) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var err error
	for c := uint8(0); c < b.config.NumRetries; c++ {
		err = b.db.Update(func(txn *badger.Txn) error {
			k := append([]byte(namespace), key...)
			// put <k, v>
			return txn.Set(k, value)
		})
		if err == nil {
			break
		}
	}
	return err
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (b *badgerDB) PutIfNotExists(namespace string, key, value []byte) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var err error
	for c := uint8(0); c < b.config.NumRetries; c++ {
		err = b.db.Update(func(txn *badger.Txn) error {
			// check if already exist
			k := append([]byte(namespace), key...)
			_, err := txn.Get(k)
			if err == nil {
				return ErrAlreadyExist
			}
			if err != badger.ErrKeyNotFound {
				return err
			}
			// put <k, v>
			return txn.Set(k, value)
		})
		if err == nil {
			break
		}
	}
	return err
}

// Get retrieves a record
func (b *badgerDB) Get(namespace string, key []byte) ([]byte, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var value []byte
	err := b.db.View(func(txn *badger.Txn) error {
		k := append([]byte(namespace), key...)
		item, err := txn.Get(k)
		if err != nil {
			return errors.Wrapf(err, "failed to get key = %x", k)
		}
		value, err = item.ValueCopy(nil)
		if err != nil {
			return errors.Wrapf(err, "failed to get value from key = %x", k)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Delete deletes a record
func (b *badgerDB) Delete(namespace string, key []byte) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var err error
	for c := uint8(0); c < b.config.NumRetries; c++ {
		err = b.db.Update(func(txn *badger.Txn) error {
			k := append([]byte(namespace), key...)
			return txn.Delete(k)
		})
		if err == nil {
			break
		}
	}
	return err
}

// Commit commits a batch
func (b *badgerDB) Commit(batch KVStoreBatch) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	succeed := false
	batch.Lock()
	defer func() {
		if succeed {
			// clear the batch if commit succeeds
			batch.ClearAndUnlock()
		} else {
			batch.Unlock()
		}

	}()

	var err error
	for c := uint8(0); c < b.config.NumRetries; c++ {
		err = b.db.Update(func(txn *badger.Txn) error {
			for i := 0; i < batch.Size(); i++ {
				write, err := batch.Entry(i)
				if err != nil {
					return err
				}
				k := append([]byte(write.namespace), write.key...)

				if write.writeType == Put {
					if err := txn.Set(k, write.value); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				} else if write.writeType == PutIfNotExists {
					_, err := txn.Get(k)
					if err == nil {
						return ErrAlreadyExist
					}
					if err != badger.ErrKeyNotFound {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
					// put <k, v>
					if err := txn.Set(k, write.value); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				} else if write.writeType == Delete {
					if err := txn.Delete(k); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				}
			}
			return nil
		})
		if err == nil || err == ErrAlreadyExist {
			break
		}
	}
	succeed = (err == nil)
	return err
}

//======================================
// private functions
//======================================

// intentionally fail to test DB can successfully rollback
func (b *badgerDB) batchPutForceFail(namespace string, key [][]byte, value [][]byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		if len(key) != len(value) {
			return errors.Wrap(ErrInvalidDB, "batch put <k, v> size not match")
		}
		for i := 0; i < len(key); i++ {
			k := []byte(namespace)
			k = append(k, []byte(key[i])...)
			if err := txn.Set(k, value[i]); err != nil {
				return err
			}
			// intentionally fail to test DB can successfully rollback
			if i == len(key)-1 {
				return errors.Wrapf(ErrInvalidDB, "force fail to test DB rollback")
			}
		}
		return nil
	})
}
