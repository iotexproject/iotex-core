// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/iotexproject/iotex-core/pkg/lifecycle"

	"github.com/iotexproject/iotex-core/db/batch"
)

type (
	// Condition spells the condition for <k, v> to be filtered out
	Condition func(k, v []byte) bool

	// KVStoreBasic is the interface of basic KV store.
	KVStoreBasic interface {
		lifecycle.StartStopper

		// Put insert or update a record identified by (namespace, key)
		Put(string, []byte, []byte) error
		// Get gets a record by (namespace, key)
		Get(string, []byte) ([]byte, error)
		// Delete deletes a record by (namespace, key)
		Delete(string, []byte) error
	}

	// KVStore is a KVStore with WriteBatch API
	KVStore interface {
		KVStoreBasic
		// WriteBatch commits a batch
		WriteBatch(batch.KVStoreBatch) error
		// Filter returns <k, v> pair in a bucket that meet the condition
		Filter(string, Condition, []byte, []byte) ([][]byte, [][]byte, error)
	}

	// KVStoreWithRange is KVStore with Range() API
	KVStoreWithRange interface {
		KVStore
		// Range gets a range of records by (namespace, key, count)
		Range(string, []byte, uint64) ([][]byte, error)
	}

	// KVStoreForRangeIndex is KVStore for range index
	KVStoreForRangeIndex interface {
		KVStore
		// Insert inserts a value into the index
		Insert([]byte, uint64, []byte) error
		// SeekNext returns value by the key (if key not exist, use next key)
		SeekNext([]byte, uint64) ([]byte, error)
		// Remove removes an existing key
		Remove([]byte, uint64) error
		// Purge deletes an existing key and all keys before it
		Purge([]byte, uint64) error
		// GetBucketByPrefix retrieves all bucket those with const namespace prefix
		GetBucketByPrefix([]byte) ([][]byte, error)
		// GetKeyByPrefix retrieves all keys those with const prefix
		GetKeyByPrefix(namespace, prefix []byte) ([][]byte, error)
		// SeekPrev returns value by the key (if key not exist, use previous key)
		SeekPrev([]byte, uint64) ([]byte, error)
	}
)
