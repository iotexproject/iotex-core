// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"

	"github.com/iotexproject/iotex-core/v2/db/batch"
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

	// KVStoreWithRangeScan is a KVStore that supports ordered range iteration.
	//
	// ScanRange is deliberately NOT the same as Filter(), and the differences are
	// consensus-relevant. Every implementation MUST obey the following semantics
	// exactly, because a divergence between storage engines (bolt vs pebble) is a
	// chain fork:
	//
	//  1. the interval is HALF-OPEN [min, max): a key equal to min is included, a
	//     key equal to max is excluded. (Filter()'s max is inclusive.) This lets a
	//     caller tile adjacent ranges without overlap or gaps.
	//  2. min == nil means "from the start of the namespace"; max == nil means
	//     "to the end of the namespace". A non-nil but empty max ([]byte{}) is a
	//     genuine upper bound and yields an empty result, since no key sorts
	//     before it.
	//  3. limit <= 0 means unlimited; otherwise at most limit pairs are returned,
	//     taken from the START of the ascending order.
	//  4. keys are returned in ascending bytes.Compare(k) order, and values[i]
	//     corresponds to keys[i].
	//  5. an empty result is (nil, nil, nil) -- NOT ErrNotExist. A namespace that
	//     does not exist is also an empty result, not an error. Callers scan
	//     ranges that are legitimately empty, so forcing them to distinguish
	//     "empty" from "missing" is a bug factory.
	//  6. if min >= max (with max != nil) the result is empty.
	//  7. returned slices are copies owned by the caller.
	KVStoreWithRangeScan interface {
		KVStore
		// ScanRange returns up to limit <k, v> pairs in [min, max), ascending by bytes.Compare(k)
		ScanRange(ns string, min, max []byte, limit int) (keys [][]byte, values [][]byte, err error)
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
