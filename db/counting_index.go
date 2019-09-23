// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// ZeroIndex is 8-byte of 0
	ZeroIndex = byteutil.Uint64ToBytesBigEndian(0)
	// ErrInvalid indicates an invalid input
	ErrInvalid = errors.New("invalid input")
)

type (
	// CountingIndex is a bucket of <k, v> where
	// k consists of 8-byte whose value increments (0, 1, 2 ... n) upon each insertion
	// position 0 (k = 0x0000000000000000) stores the total number of items in bucket so far
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
	}

	// countingIndex is CountingIndex implementation based on bolt DB
	countingIndex struct {
		db         *bolt.DB
		numRetries uint8
		bucket     []byte
		size       uint64 // total number of keys
		batch      KVStoreBatch
	}

	// memCountingIndex is the in-memory implementation of CountingIndex
	memCountingIndex struct {
		kvstore *memKVStore
		bucket  []byte
		size    uint64 // total number of keys
		batch   KVStoreBatch
	}
)

// NewCountingIndex creates a new instance of countingIndex
func NewCountingIndex(db *bolt.DB, retry uint8, name []byte, size uint64) (CountingIndex, error) {
	if db == nil {
		return nil, errors.Wrap(ErrInvalid, "db object is nil")
	}

	if len(name) == 0 {
		return nil, errors.Wrap(ErrInvalid, "bucket name is nil")
	}

	bucket := make([]byte, len(name))
	copy(bucket, name)
	return &countingIndex{
		db:         db,
		numRetries: retry,
		bucket:     bucket,
		size:       size,
	}, nil
}

// Size returns the total number of keys so far
func (c *countingIndex) Size() uint64 {
	return c.size
}

// Add inserts a value into the index
func (c *countingIndex) Add(value []byte, batch bool) error {
	if batch {
		return c.addBatch(value)
	}
	if c.batch != nil {
		return errors.Wrap(ErrInvalid, "cannot call Add in batch mode, call Commit() first to exit batch mode")
	}
	var err error
	for i := uint8(0); i < c.numRetries; i++ {
		if err = c.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(c.bucket)
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", c.bucket)
			}
			last := byteutil.Uint64ToBytesBigEndian(c.size + 1)
			if err := bucket.Put(last, value); err != nil {
				return err
			}
			// update the total amount
			return bucket.Put(ZeroIndex, last)
		}); err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	c.size++
	return nil
}

// addBatch inserts a value into the index in batch mode
func (c *countingIndex) addBatch(value []byte) error {
	if c.batch == nil {
		c.batch = NewBatch()
	}
	c.batch.Put("", byteutil.Uint64ToBytesBigEndian(c.size+1), value, "failed to put")
	c.size++
	return nil
}

// Get return value of key[slot]
func (c *countingIndex) Get(slot uint64) ([]byte, error) {
	if slot >= c.size {
		return nil, errors.Wrapf(ErrNotExist, "slot: %d", slot)
	}

	var value []byte
	err := c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", c.bucket)
		}
		// seek to start
		cur := bucket.Cursor()
		k, v := cur.Seek(byteutil.Uint64ToBytesBigEndian(slot + 1))
		if k == nil {
			return errors.Wrapf(ErrNotExist, "entry at %d doesn't exist", slot)
		}
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Range return value of keys [start, start+count)
func (c *countingIndex) Range(start, count uint64) ([][]byte, error) {
	if start+count > c.size || count == 0 {
		return nil, errors.Wrapf(ErrInvalid, "start: %d, count: %d", start, count)
	}

	value := make([][]byte, count)
	err := c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", c.bucket)
		}
		// seek to start
		cur := bucket.Cursor()
		k, v := cur.Seek(byteutil.Uint64ToBytesBigEndian(start + 1))
		if k == nil {
			return errors.Wrapf(ErrNotExist, "entry at %d doesn't exist", start)
		}

		// retrieve 'count' items
		for i := uint64(0); i < count; k, v = cur.Next() {
			if k == nil {
				return errors.Wrapf(ErrNotExist, "entry at %d doesn't exist", start+1+i)
			}
			value[i] = make([]byte, len(v))
			copy(value[i], v)
			i++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Revert removes entries from end
func (c *countingIndex) Revert(count uint64) error {
	if c.batch != nil {
		return errors.Wrap(ErrInvalid, "cannot call Revert in batch mode, call Commit() first to exit batch mode")
	}
	if count == 0 || count > c.size {
		return errors.Wrapf(ErrInvalid, "count: %d", count)
	}

	return c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", c.bucket)
		}

		start := c.size - count
		// seek to start
		cur := bucket.Cursor()
		k, _ := cur.Seek(byteutil.Uint64ToBytesBigEndian(start + 1))
		if k == nil {
			return errors.Wrapf(ErrNotExist, "entry at %d doesn't exist", start)
		}

		// delete 'count' items
		for i := uint64(0); i < count; k, _ = cur.Next() {
			if k == nil {
				return errors.Wrapf(ErrNotExist, "entry at %d doesn't exist", start+i)
			}
			if err := bucket.Delete(k); err != nil {
				return errors.Wrapf(ErrNotExist, "failed to delete entry at %d", start+i)
			}
			i++
		}
		// update the total amount
		c.size = start
		return bucket.Put(ZeroIndex, byteutil.Uint64ToBytesBigEndian(start))
	})
}

// Close makes the index not usable
func (c *countingIndex) Close() {
	// frees reference to db, the db object itself will be closed/freed by its owner, not here
	c.db = nil
	c.batch = nil
	c.bucket = nil
}

// Commit commits a batch
func (c *countingIndex) Commit() (err error) {
	if c.batch == nil {
		return nil
	}

	succeed := true
	c.batch.Lock()
	defer func() {
		if succeed {
			// clear the batch if commit succeeds
			c.batch.ClearAndUnlock()
			c.batch = nil
		} else {
			c.batch.Unlock()
		}
	}()

	numRetries := c.numRetries
	for i := uint8(0); i < numRetries; i++ {
		if err = c.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(c.bucket)
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", c.bucket)
			}
			// set an aggressive fill percent
			// b/c counting index only appends, further inserts to the bucket would never split the page
			bucket.FillPercent = 1.0
			for i := 0; i < c.batch.Size(); i++ {
				write, err := c.batch.Entry(i)
				if err != nil {
					return err
				}
				if err := bucket.Put(write.key, write.value); err != nil {
					return errors.Wrapf(err, write.errorFormat, write.errorArgs)
				}
			}
			// update the total amount
			return bucket.Put(ZeroIndex, byteutil.Uint64ToBytesBigEndian(c.size))
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

// NewInMemCountingIndex creates a new instance of memCountingIndex
func NewInMemCountingIndex(mem *memKVStore, name []byte, size uint64) (CountingIndex, error) {
	if mem == nil {
		return nil, errors.Wrap(ErrInvalid, "memKVStore object is nil")
	}
	if len(name) == 0 {
		return nil, errors.Wrap(ErrInvalid, "bucket name is nil")
	}
	bucket := make([]byte, len(name))
	copy(bucket, name)
	return &memCountingIndex{
		kvstore: mem,
		bucket:  bucket,
		size:    size,
	}, nil
}

// Size returns the total number of keys so far
func (m *memCountingIndex) Size() uint64 {
	return m.size
}

// Add inserts a value into the index
func (m *memCountingIndex) Add(value []byte, batch bool) error {
	if err := m.checkBucket(); err != nil {
		return err
	}
	if batch {
		return m.addBatch(value)
	}
	if m.batch != nil {
		return errors.Wrap(ErrInvalid, "cannot call Add in batch mode, call Commit() first to exit batch mode")
	}
	last := byteutil.Uint64ToBytesBigEndian(m.size + 1)
	if err := m.kvstore.Put(string(m.bucket), last, value); err != nil {
		return err
	}
	// update the total amount
	if err := m.kvstore.Put(string(m.bucket), ZeroIndex, last); err != nil {
		return err
	}
	m.size++
	return nil
}

// addBatch inserts a value into the index in batch mode
func (m *memCountingIndex) addBatch(value []byte) error {
	if m.batch == nil {
		m.batch = NewBatch()
	}
	m.batch.Put(string(m.bucket), byteutil.Uint64ToBytesBigEndian(m.size+1), value, "failed to put")
	m.size++
	return nil
}

// Get return value of key[slot]
func (m *memCountingIndex) Get(slot uint64) ([]byte, error) {
	if err := m.checkBucket(); err != nil {
		return nil, err
	}
	if slot >= m.size {
		return nil, errors.Wrapf(ErrNotExist, "slot: %d", slot)
	}
	return m.kvstore.Get(string(m.bucket), byteutil.Uint64ToBytesBigEndian(slot+1))
}

// Range return value of keys [start, start+count)
func (m *memCountingIndex) Range(start, count uint64) ([][]byte, error) {
	if err := m.checkBucket(); err != nil {
		return nil, err
	}
	if start+count > m.size || count == 0 {
		return nil, errors.Wrapf(ErrInvalid, "start: %d, count: %d", start, count)
	}

	value := make([][]byte, count)
	for i := uint64(0); i < count; i++ {
		v, err := m.kvstore.Get(string(m.bucket), byteutil.Uint64ToBytesBigEndian(start+1+i))
		if err != nil {
			return nil, errors.Wrapf(ErrNotExist, "entry at %d doesn't exist", start+1+i)
		}
		value[i] = make([]byte, len(v))
		copy(value[i], v)
	}
	return value, nil
}

// Revert removes entries from end
func (m *memCountingIndex) Revert(count uint64) error {
	if err := m.checkBucket(); err != nil {
		return err
	}
	if m.batch != nil {
		return errors.Wrap(ErrInvalid, "cannot call Revert in batch mode, call Commit() first to exit batch mode")
	}
	if count == 0 || count > m.size {
		return errors.Wrapf(ErrInvalid, "count: %d", count)
	}

	start := m.size - count
	for i := uint64(0); i < count; i++ {
		if err := m.kvstore.Delete(string(m.bucket), byteutil.Uint64ToBytesBigEndian(start+1+i)); err != nil {
			return err
		}
	}
	// update the total amount
	m.size = start
	return m.kvstore.Put(string(m.bucket), ZeroIndex, byteutil.Uint64ToBytesBigEndian(start))
}

// Close makes the index not usable
func (m *memCountingIndex) Close() {
	// frees reference to KVStore, the KVStore object itself will be closed/freed by its owner, not here
	m.kvstore = nil
	m.batch = nil
	m.bucket = nil
}

// Commit commits a batch
func (m *memCountingIndex) Commit() error {
	if err := m.checkBucket(); err != nil {
		return err
	}
	if m.batch == nil {
		return nil
	}
	// commit the batch
	if err := m.kvstore.Commit(m.batch); err != nil {
		return err
	}
	m.batch = nil
	// update the total amount
	return m.kvstore.Put(string(m.bucket), ZeroIndex, byteutil.Uint64ToBytesBigEndian(m.size))
}

func (m *memCountingIndex) checkBucket() error {
	if _, ok := m.kvstore.bucket.Load(string(m.bucket)); !ok {
		return errors.Wrapf(ErrBucketNotExist, "bucket = %s doesn't exist", m.bucket)
	}
	return nil
}
