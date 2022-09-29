// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"fmt"
	"sync/atomic"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// CountKey is the special key to store the size of index, it satisfies bytes.Compare(CountKey, MinUint64) = -1
	CountKey = []byte{0, 0, 0, 0, 0, 0, 0}
	// ZeroIndex is 8-byte of 0
	ZeroIndex = byteutil.Uint64ToBytesBigEndian(0)
	// ErrInvalid indicates an invalid input
	ErrInvalid = errors.New("invalid input")
)

type (
	// CountingIndex is a bucket of <k, v> where
	// k consists of 8-byte whose value increments (0, 1, 2 ... n) upon each insertion
	// the special key CountKey stores the total number of items in bucket so far
	CountingIndex interface {
		// Size returns the total number of keys so far
		Size() uint64
		// Add inserts a value into the index
		Add([]byte, bool) error
		// Get return value of key[slot]
		Get(uint64) ([]byte, error)
		// Range return value of keys [start, start+count)
		Range(uint64, uint64) ([][]byte, error)
		// Revert removes entries from end
		Revert(uint64) error
		// Close makes the index not usable
		Close()
		// Commit commits the batch
		Commit() error
		// UseBatch
		UseBatch(batch.KVStoreBatch) error
		// Finalize
		Finalize() error
	}

	// countingIndex is CountingIndex implementation based on KVStore
	countingIndex struct {
		kvStore KVStoreWithRange
		bucket  string
		size    uint64 // total number of keys
		batch   batch.KVStoreBatch
	}
)

// NewCountingIndexNX creates a new counting index if it does not exist, otherwise return existing index
func NewCountingIndexNX(kv KVStore, name []byte) (CountingIndex, error) {
	if kv == nil {
		return nil, errors.Wrap(ErrInvalid, "KVStore object is nil")
	}
	kvRange, ok := kv.(KVStoreWithRange)
	if !ok {
		return nil, errors.New("counting index can only be created from KVStoreWithRange")
	}
	if len(name) == 0 {
		return nil, errors.Wrap(ErrInvalid, "bucket name is nil")
	}
	bucket := string(name)
	// check if the index exist or not
	total, err := kv.Get(bucket, CountKey)
	if errors.Cause(err) == ErrNotExist || total == nil {
		// put 0 as total number of keys
		if err := kv.Put(bucket, CountKey, ZeroIndex); err != nil {
			return nil, errors.Wrapf(err, "failed to create counting index %x", name)
		}
		total = ZeroIndex
	}

	idx := &countingIndex{
		kvStore: kvRange,
		bucket:  bucket,
	}
	atomic.StoreUint64(&idx.size, byteutil.BytesToUint64BigEndian(total))
	return idx, nil
}

// GetCountingIndex return an existing counting index
func GetCountingIndex(kv KVStore, name []byte) (CountingIndex, error) {
	kvRange, ok := kv.(KVStoreWithRange)
	if !ok {
		return nil, errors.New("counting index can only be created from KVStoreWithRange")
	}
	bucket := string(name)
	// check if the index exist or not
	total, err := kv.Get(bucket, CountKey)
	if errors.Cause(err) == ErrNotExist || total == nil {
		return nil, errors.Wrapf(err, "counting index 0x%x doesn't exist", name)
	}
	idx := &countingIndex{
		kvStore: kvRange,
		bucket:  bucket,
	}
	atomic.StoreUint64(&idx.size, byteutil.BytesToUint64BigEndian(total))
	return idx, nil
}

// Size returns the total number of keys so far
func (c *countingIndex) Size() uint64 {
	return atomic.LoadUint64(&c.size)
}

// Add inserts a value into the index
func (c *countingIndex) Add(value []byte, inBatch bool) error {
	if inBatch {
		return c.addBatch(value)
	}
	if c.batch != nil {
		return errors.Wrap(ErrInvalid, "cannot call Add in batch mode, call Commit() first to exit batch mode")
	}
	b := batch.NewBatch()
	size := atomic.LoadUint64(&c.size)
	b.Put(c.bucket, byteutil.Uint64ToBytesBigEndian(size), value, fmt.Sprintf("failed to add %d-th item", size+1))
	b.Put(c.bucket, CountKey, byteutil.Uint64ToBytesBigEndian(size+1), fmt.Sprintf("failed to update size = %d", size+1))
	b.AddFillPercent(c.bucket, 1.0)
	if err := c.kvStore.WriteBatch(b); err != nil {
		return err
	}
	atomic.AddUint64(&c.size, 1)
	return nil
}

// addBatch inserts a value into the index in batch mode
func (c *countingIndex) addBatch(value []byte) error {
	if c.batch == nil {
		c.batch = batch.NewBatch()
	}
	size := atomic.LoadUint64(&c.size)
	c.batch.Put(c.bucket, byteutil.Uint64ToBytesBigEndian(size), value, fmt.Sprintf("failed to add %d-th item", size+1))
	atomic.AddUint64(&c.size, 1)
	return nil
}

// Get return value of key[slot]
func (c *countingIndex) Get(slot uint64) ([]byte, error) {
	if slot >= atomic.LoadUint64(&c.size) {
		return nil, errors.Wrapf(ErrNotExist, "slot: %d", slot)
	}
	return c.kvStore.Get(c.bucket, byteutil.Uint64ToBytesBigEndian(slot))
}

// Range return value of keys [start, start+count)
func (c *countingIndex) Range(start, count uint64) ([][]byte, error) {
	if start+count > atomic.LoadUint64(&c.size) || count == 0 {
		return nil, errors.Wrapf(ErrInvalid, "start: %d, count: %d", start, count)
	}
	return c.kvStore.Range(c.bucket, byteutil.Uint64ToBytesBigEndian(start), count)
}

// Revert removes entries from end
func (c *countingIndex) Revert(count uint64) error {
	if c.batch != nil {
		return errors.Wrap(ErrInvalid, "cannot call Revert in batch mode, call Commit() first to exit batch mode")
	}
	size := atomic.LoadUint64(&c.size)
	if count == 0 || count > size {
		return errors.Wrapf(ErrInvalid, "count: %d", count)
	}
	b := batch.NewBatch()
	start := size - count
	for i := uint64(0); i < count; i++ {
		b.Delete(c.bucket, byteutil.Uint64ToBytesBigEndian(start+i), fmt.Sprintf("failed to delete %d-th item", start+i))
	}
	b.Put(c.bucket, CountKey, byteutil.Uint64ToBytesBigEndian(start), fmt.Sprintf("failed to update size = %d", start))
	b.AddFillPercent(c.bucket, 1.0)
	if err := c.kvStore.WriteBatch(b); err != nil {
		return err
	}
	atomic.StoreUint64(&c.size, start)
	return nil
}

// Close makes the index not usable
func (c *countingIndex) Close() {
	// frees reference to db, the db object itself will be closed/freed by its owner, not here
	c.kvStore = nil
	c.batch = nil
}

// Commit commits a batch
func (c *countingIndex) Commit() error {
	if c.batch == nil {
		return nil
	}
	size := atomic.LoadUint64(&c.size)
	c.batch.Put(c.bucket, CountKey, byteutil.Uint64ToBytesBigEndian(size), fmt.Sprintf("failed to update size = %d", size))
	c.batch.AddFillPercent(c.bucket, 1.0)
	if err := c.kvStore.WriteBatch(c.batch); err != nil {
		return err
	}
	c.batch = nil
	return nil
}

// UseBatch sets a (usually common) batch for the counting index to use
func (c *countingIndex) UseBatch(b batch.KVStoreBatch) error {
	if b == nil {
		return ErrInvalid
	}
	c.batch = b
	return nil
}

// Finalize updates the total size before committing the (usually common) batch
func (c *countingIndex) Finalize() error {
	if c.batch == nil {
		return ErrInvalid
	}
	size := atomic.LoadUint64(&c.size)
	c.batch.Put(c.bucket, CountKey, byteutil.Uint64ToBytesBigEndian(size), fmt.Sprintf("failed to update size = %d", size))
	c.batch.AddFillPercent(c.bucket, 1.0)
	c.batch = nil
	return nil
}
