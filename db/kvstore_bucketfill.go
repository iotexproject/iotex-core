// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

type (
	// KVStoreWithBucketFillPercent is KVStore with option to set bucket fill percent
	KVStoreWithBucketFillPercent interface {
		KVStore
		// SetBucketFillPercent sets specified fill percent for a bucket
		SetBucketFillPercent(string, float64) error
	}

	boltDBWithFillPercent struct {
		*boltDB
		fillPercent map[string]float64 // specific fill percent for certain buckets (for example, 1.0 for append-only)
	}

	memKVStoreWithFillPercent struct {
		*memKVStore
	}
)

// NewKVStoreWithBucketFillPercent instantiates an BoltDB with bucket fill option
func NewKVStoreWithBucketFillPercent(kv KVStore) KVStoreWithBucketFillPercent {
	switch v := kv.(type) {
	case *boltDB:
		return &boltDBWithFillPercent{
			boltDB:      v,
			fillPercent: make(map[string]float64),
		}
	case *memKVStore:
		return &memKVStoreWithFillPercent{
			memKVStore: v,
		}
	default:
		panic("KVStore type is not supported")
	}
}

// Put inserts a <key, value> record
func (b *boltDBWithFillPercent) Put(namespace string, key, value []byte) (err error) {
	for c := uint8(0); c < b.config.NumRetries; c++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
			if err != nil {
				return err
			}
			if p, ok := b.fillPercent[namespace]; ok {
				bucket.FillPercent = p
			}
			return bucket.Put(key, value)
		}); err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// Commit commits a batch
func (b *boltDBWithFillPercent) Commit(batch KVStoreBatch) (err error) {
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

	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			for i := 0; i < batch.Size(); i++ {
				write, err := batch.Entry(i)
				if err != nil {
					return err
				}
				if write.writeType == Put {
					bucket, err := tx.CreateBucketIfNotExists([]byte(write.namespace))
					if err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
					if p, ok := b.fillPercent[write.namespace]; ok {
						bucket.FillPercent = p
					}
					if err := bucket.Put(write.key, write.value); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				} else if write.writeType == Delete {
					bucket := tx.Bucket([]byte(write.namespace))
					if bucket == nil {
						continue
					}
					if err := bucket.Delete(write.key); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				}
			}
			return nil
		}); err == nil {
			break
		}
	}

	if err != nil {
		succeed = false
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// SetBucketFillPercent sets specified fill percent for a bucket
func (b *boltDBWithFillPercent) SetBucketFillPercent(namespace string, percent float64) error {
	b.fillPercent[namespace] = percent
	return nil
}

// SetBucketFillPercent sets specified fill percent for a bucket
func (m *memKVStoreWithFillPercent) SetBucketFillPercent(string, float64) error {
	return nil
}
