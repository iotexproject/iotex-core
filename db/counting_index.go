// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
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
		// AddToBatch inserts an entry into external batch
		AddToBatch(batch.KVStoreBatch, []byte)
		// BeginBatch should be called before writing the batch
		BeginBatch(batch.KVStoreBatch)
		// EndBatch should be called after the batch has been written to DB
		EndBatch()
		// Get return value of key[slot]
		Get(uint64) ([]byte, error)
		// Range return value of keys [start, start+count)
		Range(uint64, uint64) ([][]byte, error)
		// Revert removes entries from end
		Revert(uint64) error
		// Close makes the index not usable
		Close()
	}

	// countingIndex is CountingIndex implementation based on KVStore
	// it is designed for a single writer and multiple readers
	//
	// for writer to add entries to the CountingIndex:
	// c, err := NewCountingIndexNX(kv, name)
	// b := batch.NewBatch()
	// c.AddToBatch(b, []byte("entry 1"))
	// c.AddToBatch(b, []byte("entry 2"))
	// ...
	// c.AddToBatch(b, []byte("entry n"))
	// c.BeginBatch()
	// write the batch to underlying DB
	// c.EndBatch()
	//
	// multiple readers can call reader methods concurrently:
	// size := c.Size()
	// v, err := c.Get(1)
	// entries, err := c.Range(1, 100)
	countingIndex struct {
		lock       sync.RWMutex
		kvStore    KVStoreWithRange
		bucket     string
		size       uint64
		writerSize uint64
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
	var (
		bucket = string(name)
		size   uint64
	)
	// check if the index exist or not
	total, err := kv.Get(bucket, CountKey)
	if errors.Cause(err) == ErrNotExist || total == nil {
		// put 0 as total number of keys
		if err := kv.Put(bucket, CountKey, ZeroIndex); err != nil {
			return nil, errors.Wrapf(err, "failed to create counting index %x", name)
		}
	} else {
		size = byteutil.BytesToUint64BigEndian(total)
	}
	return &countingIndex{
		kvStore:    kvRange,
		bucket:     bucket,
		size:       size,
		writerSize: size,
	}, nil
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
	size := byteutil.BytesToUint64BigEndian(total)
	return &countingIndex{
		kvStore:    kvRange,
		bucket:     bucket,
		size:       size,
		writerSize: size,
	}, nil
}

// Size returns the total number of keys so far
func (c *countingIndex) Size() uint64 {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.size
}

func (c *countingIndex) AddToBatch(b batch.KVStoreBatch, value []byte) {
	b.Put(c.bucket, byteutil.Uint64ToBytesBigEndian(c.writerSize), value, fmt.Sprintf("failed to add %d-th item", c.writerSize))
	b.Put(c.bucket, CountKey, byteutil.Uint64ToBytesBigEndian(c.writerSize+1), fmt.Sprintf("failed to update size = %d", c.writerSize+1))
	c.writerSize++
}

func (c *countingIndex) BeginBatch(b batch.KVStoreBatch) {
	c.lock.Lock()
	b.AddFillPercent(c.bucket, 1.0)
}

func (c *countingIndex) EndBatch() {
	c.size = c.writerSize
	c.lock.Unlock()
}

// Get return value of key[slot]
func (c *countingIndex) Get(slot uint64) ([]byte, error) {
	c.lock.RLock()
	if slot >= c.size {
		c.lock.RUnlock()
		return nil, errors.Wrapf(ErrNotExist, "slot: %d", slot)
	}
	c.lock.RUnlock()
	return c.kvStore.Get(c.bucket, byteutil.Uint64ToBytesBigEndian(slot))
}

// Range return value of keys [start, start+count)
func (c *countingIndex) Range(start, count uint64) ([][]byte, error) {
	c.lock.RLock()
	if start+count > c.size || count == 0 {
		c.lock.RUnlock()
		return nil, errors.Wrapf(ErrInvalid, "start: %d, count: %d", start, count)
	}
	c.lock.RUnlock()
	return c.kvStore.Range(c.bucket, byteutil.Uint64ToBytesBigEndian(start), count)
}

// Revert removes entries from end
func (c *countingIndex) Revert(count uint64) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if count == 0 || count > c.size {
		return errors.Wrapf(ErrInvalid, "count: %d", count)
	}
	b := batch.NewBatch()
	start := c.size - count
	for i := uint64(0); i < count; i++ {
		b.Delete(c.bucket, byteutil.Uint64ToBytesBigEndian(start+i), fmt.Sprintf("failed to delete %d-th item", start+i))
	}
	b.Put(c.bucket, CountKey, byteutil.Uint64ToBytesBigEndian(start), fmt.Sprintf("failed to update size = %d", start))
	b.AddFillPercent(c.bucket, 1.0)
	if err := c.kvStore.WriteBatch(b); err != nil {
		return err
	}
	c.size = start
	c.writerSize = start
	return nil
}

// Close makes the index not usable
func (c *countingIndex) Close() {
	// frees reference to db, the db object itself will be closed/freed by its owner, not here
	c.kvStore = nil
}
