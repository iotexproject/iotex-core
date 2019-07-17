// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"

	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
)

// badgerDB is KVStore implementation based bolt DB
type badgerDB struct {
	db     *badger.DB
	path   string
	config config.DB
}

// Start opens the badgerDB (creates new file if not existing yet)
func (b *badgerDB) Start(_ context.Context) error {
	opts := badger.DefaultOptions
	opts.Dir = b.path
	opts.ValueDir = b.path
	db, err := badger.Open(opts)
	if err != nil {
		return errors.Wrap(ErrIO, err.Error())
	}
	b.db = db
	return nil
}

// Stop closes the badgerDB
func (b *badgerDB) Stop(_ context.Context) error {
	if b.db != nil {
		if err := b.db.Close(); err != nil {
			return errors.Wrap(ErrIO, err.Error())
		}
	}
	return nil
}

// Put inserts a <key, value> record
func (b *badgerDB) Put(namespace string, key, value []byte) (err error) {
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
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// Get retrieves a record
func (b *badgerDB) Get(namespace string, key []byte) ([]byte, error) {
	var value []byte
	err := b.db.View(func(txn *badger.Txn) error {
		k := append([]byte(namespace), key...)
		item, err := txn.Get(k)
		if err != nil {
			return errors.Wrapf(err, "failed to get key = %x", k)
		}
		if value, err = item.ValueCopy(nil); err != nil {
			return errors.Wrapf(err, "failed to get value from key = %x", k)
		}
		return nil
	})
	if err == nil {
		return value, nil
	}
	if err == badger.ErrKeyNotFound {
		return nil, errors.Wrap(ErrNotExist, err.Error())
	}
	return nil, errors.Wrap(ErrIO, err.Error())
}

// Delete deletes a record
func (b *badgerDB) Delete(namespace string, key []byte) (err error) {
	for c := uint8(0); c < b.config.NumRetries; c++ {
		err = b.db.Update(func(txn *badger.Txn) error {
			k := append([]byte(namespace), key...)
			return txn.Delete(k)
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// Commit commits a batch
func (b *badgerDB) Commit(batch KVStoreBatch) (err error) {
	succeed := true
	batch.Lock()
	defer func() {
		if succeed {
			// clear the batch if commit succeeds
			batch.ClearAndUnlock()
		} else {
			batch.Unlock()
		}

	}()

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
	if err != nil {
		succeed = false
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}
