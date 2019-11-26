// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// MaxKey is the special key such that bytes.Compare(MaxUint64, MaxKey) = -1
	MaxKey = []byte{255, 255, 255, 255, 255, 255, 255, 255, 0}
	// NotExist is the empty byte slice to indicate a key does not exist (as a result of calling Purge())
	NotExist = []byte{}
)

type (
	// RangeIndex is a bucket of sparse <k, v> pair, where k consists of 8-byte value
	// and all keys that falls in 2 consecutive k have the same v
	// for example, given 3 entries in the bucket:
	//
	// k = 0x0000000000000004 ==> v1
	// k = 0x0000000000000123 ==> v2
	// k = 0x0000000000005678 ==> v3
	//
	// we have:
	// for all key   0x0 <= k <  0x4,    value[k] = initial value
	// for all key   0x4 <= k <  0x123,  value[k] = v1
	// for all key 0x123 <= k <  0x5678, value[k] = v2
	// for all key          k >= 0x5678, value[k] = v3
	//
	// position 0 (k = 0x0000000000000000) stores the value to return beyond the largest inserted key
	//
	RangeIndex interface {
		// Insert inserts a value into the index
		Insert(uint64, []byte) error
		// Get returns value by the key
		Get(uint64) ([]byte, error)
		// Delete deletes an existing key
		Delete(uint64) error
		// Purge deletes an existing key and all keys before it
		Purge(uint64) error
		// Close makes the index not usable
		Close()
	}

	// rangeIndex is RangeIndex implementation based on bolt DB
	rangeIndex struct {
		kvstore KVStoreForRangeIndex
		bucket  []byte
	}
)

// NewRangeIndex creates a new instance of rangeIndex
func NewRangeIndex(kv KVStore, name, init []byte) (RangeIndex, error) {
	if kv == nil {
		return nil, errors.Wrap(ErrInvalid, "KVStore object is nil")
	}
	kvRange, ok := kv.(KVStoreForRangeIndex)
	if !ok {
		return nil, errors.Wrap(ErrInvalid, "range index can only be created from KVStoreForRangeIndex")
	}
	if len(name) == 0 {
		return nil, errors.Wrap(ErrInvalid, "bucket name is nil")
	}
	// check whether init value exist or not
	v, err := kv.Get(string(name), MaxKey)
	if errors.Cause(err) == ErrNotExist || v == nil {
		// write the initial value
		if err := kv.Put(string(name), MaxKey, init); err != nil {
			return nil, errors.Wrapf(err, "failed to create range index %x", name)
		}
	}

	bucket := make([]byte, len(name))
	copy(bucket, name)

	return &rangeIndex{
		kvstore: kvRange,
		bucket:  bucket,
	}, nil
}

// Insert inserts a value into the index
func (r *rangeIndex) Insert(key uint64, value []byte) error {
	return r.kvstore.Insert(r.bucket, key, value)
}

// Get returns value by the key
func (r *rangeIndex) Get(key uint64) ([]byte, error) {
	return r.kvstore.Seek(r.bucket, key)
}

// Delete deletes an existing key
func (r *rangeIndex) Delete(key uint64) error {
	return r.kvstore.Remove(r.bucket, key)
}

// Purge deletes an existing key and all keys before it
func (r *rangeIndex) Purge(key uint64) error {
	return r.kvstore.Purge(r.bucket, key)
}

// Close makes the index not usable
func (r *rangeIndex) Close() {
	// frees reference to db, the db object itself will be closed/freed by its owner, not here
	r.kvstore = nil
	r.bucket = nil
}

// memRangeIndex is RangeIndex implementation based on memKVStore
type memRangeIndex struct {
	db     KVStore
	bucket []byte
}

// NewMemRangeIndex creates a new instance of rangeIndex
func NewMemRangeIndex(db KVStore, retry uint8, name []byte) (RangeIndex, error) {
	if db == nil {
		return nil, errors.Wrap(ErrInvalid, "db object is nil")
	}

	if len(name) == 0 {
		return nil, errors.Wrap(ErrInvalid, "bucket name is nil")
	}

	bucket := make([]byte, len(name))
	copy(bucket, name)

	return &memRangeIndex{
		db:     db,
		bucket: bucket,
	}, nil
}

// Insert inserts a value into the index
func (r *memRangeIndex) Insert(key uint64, value []byte) error {
	return r.db.Put(string(r.bucket), byteutil.Uint64ToBytesBigEndian(key), value)
}

// Get returns value by the key
func (r *memRangeIndex) Get(key uint64) ([]byte, error) {
	return r.db.Get(string(r.bucket), byteutil.Uint64ToBytesBigEndian(key))
}

// Delete deletes an existing key
func (r *memRangeIndex) Delete(key uint64) error {
	return r.db.Delete(string(r.bucket), byteutil.Uint64ToBytesBigEndian(key))
}

// Purge deletes an existing key and all keys before it
func (r *memRangeIndex) Purge(key uint64) error {
	for i := uint64(0); i < key; i++ {
		r.Delete(i)
	}
	return nil
}

// Close makes the index not usable
func (r *memRangeIndex) Close() {
}
