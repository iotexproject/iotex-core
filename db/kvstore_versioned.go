// Copyright (c) 2024 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
)

type (
	// KvVersioned is a versioned key-value store, where each key has multiple
	// versions of value (corresponding to different heights in a blockchain)
	//
	// Versioning is achieved by using (key + 8-byte version) as the actual
	// storage key in the underlying DB. For each bucket, a metadata is stored
	// at the special key = []byte{0}. The metadata specifies the bucket's name
	// and the key length.
	//
	// For each versioned key, the special location = key + []byte{0} stores the
	// key's metadata, which includes the following info:
	// 1. the version when the key is first created
	// 2. the version when the key is lastly written
	// 3. the version when the key is deleted
	// 4. the key's last written value (to fast-track read of current version)
	// If the location does not store a value, the key has never been written.
	//
	// How to use a versioned key-value store:
	//
	// db := NewKVStoreWithVersion(cfg) // creates a versioned DB
	// db.Start(ctx)
	// defer func() { db.Stop(ctx) }()
	//
	// kv := db.SetVersion(5)
	// value, err := kv.Get("ns", key) // read 'key' at version 5
	// kv = db.SetVersion(8)
	// err := kv.Put("ns", key, value) // write 'key' at version 8

	KvVersioned interface {
		lifecycle.StartStopper

		// Version returns the key's most recent version
		Version(string, []byte) (uint64, error)

		// SetVersion sets the version, and returns a KVStore to call Put()/Get()
		SetVersion(uint64) KVStore
	}

	// KvWithVersion wraps the versioned DB implementation with a certain version
	KvWithVersion struct {
		db      VersionedDB
		version uint64 // the current version
	}
)

// Option sets an option
type Option func(*KvWithVersion)

// NewKVStoreWithVersion implements a KVStore that can handle both versioned
// and non-versioned namespace
func NewKVStoreWithVersion(cfg Config, opts ...Option) *KvWithVersion {
	db := NewBoltDBVersioned(cfg)
	kv := KvWithVersion{
		db: db,
	}
	for _, opt := range opts {
		opt(&kv)
	}
	return &kv
}

// Start starts the DB
func (b *KvWithVersion) Start(ctx context.Context) error {
	return b.db.Start(ctx)
}

// Stop stops the DB
func (b *KvWithVersion) Stop(ctx context.Context) error {
	return b.db.Stop(ctx)
}

// Put writes a <key, value> record
func (b *KvWithVersion) Put(ns string, key, value []byte) error {
	return b.db.Put(b.version, ns, key, value)
}

// Get retrieves a key's value
func (b *KvWithVersion) Get(ns string, key []byte) ([]byte, error) {
	return b.db.Get(b.version, ns, key)
}

// Delete deletes a key
func (b *KvWithVersion) Delete(ns string, key []byte) error {
	return b.db.Delete(b.version, ns, key)
}

// Filter returns <k, v> pair in a bucket that meet the condition
func (b *KvWithVersion) Filter(ns string, cond Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	return b.db.Filter(b.version, ns, cond, minKey, maxKey)
}

// WriteBatch commits a batch
func (b *KvWithVersion) WriteBatch(kvsb batch.KVStoreBatch) error {
	return b.db.CommitBatch(b.version, kvsb)
}

// Version returns the key's most recent version
func (b *KvWithVersion) Version(ns string, key []byte) (uint64, error) {
	return b.db.Version(ns, key)
}

// SetVersion sets the version, and returns a KVStore to call Put()/Get()
func (b *KvWithVersion) SetVersion(v uint64) KVStore {
	kv := KvWithVersion{
		db:      b.db,
		version: v,
	}
	return &kv
}
