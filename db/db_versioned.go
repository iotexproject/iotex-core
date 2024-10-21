// Copyright (c) 2024 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"

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
	// 4. hash of the key's last written value (to detect/avoid same write)
	// If the location does not store a value, the key has never been written.
	//
	// How to use a versioned DB:
	//
	// db := NewBoltDBVersioned(cfg) // creates a versioned DB
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
		SetVersion(uint64) KVStoreBasic
	}

	// BoltDBVersioned is KvVersioned implementation based on bolt DB
	BoltDBVersioned struct {
		db *BoltDB
	}
)

// Option sets an option
type Option func(b *BoltDBVersioned)

// NewBoltDBVersioned instantiates an BoltDB which implements KvVersioned
func NewBoltDBVersioned(cfg Config, opts ...Option) *BoltDBVersioned {
	b := &BoltDBVersioned{
		db: NewBoltDB(cfg),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Start starts the DB
func (b *BoltDBVersioned) Start(ctx context.Context) error {
	return b.db.Start(ctx)
}

// Stop stops the DB
func (b *BoltDBVersioned) Stop(ctx context.Context) error {
	return b.db.Stop(ctx)
}

// Put writes a <key, value> record
func (b *BoltDBVersioned) Put(ns string, version uint64, key, value []byte) error {
	if !b.db.IsReady() {
		return ErrDBNotStarted
	}
	// TODO: implement Put
	return nil
}

// Get retrieves the most recent version
func (b *BoltDBVersioned) Get(ns string, version uint64, key []byte) ([]byte, error) {
	if !b.db.IsReady() {
		return nil, ErrDBNotStarted
	}
	// TODO: implement Get
	return nil, nil
}

// Delete deletes a record,if key is nil,this will delete the whole bucket
func (b *BoltDBVersioned) Delete(ns string, key []byte) error {
	if !b.db.IsReady() {
		return ErrDBNotStarted
	}
	// TODO: implement Delete
	return nil
}

// Version returns the key's most recent version
func (b *BoltDBVersioned) Version(ns string, key []byte) (uint64, error) {
	if !b.db.IsReady() {
		return 0, ErrDBNotStarted
	}
	// TODO: implement Version
	return 0, nil
}

// SetVersion sets the version, and returns a KVStore to call Put()/Get()
func (b *BoltDBVersioned) SetVersion(v uint64) KVStoreBasic {
	return &KvWithVersion{
		db:      b,
		version: v,
	}
}

// KvWithVersion wraps the BoltDBVersioned with a certain version
type KvWithVersion struct {
	db      *BoltDBVersioned
	version uint64 // version for Get/Put()
}

// Start starts the DB
func (b *KvWithVersion) Start(context.Context) error {
	panic("should call BoltDBVersioned's Start method")
}

// Stop stops the DB
func (b *KvWithVersion) Stop(context.Context) error {
	panic("should call BoltDBVersioned's Stop method")
}

// Put writes a <key, value> record
func (b *KvWithVersion) Put(ns string, key, value []byte) error {
	return b.db.Put(ns, b.version, key, value)
}

// Get retrieves a key's value
func (b *KvWithVersion) Get(ns string, key []byte) ([]byte, error) {
	return b.db.Get(ns, b.version, key)
}

// Delete deletes a key
func (b *KvWithVersion) Delete(ns string, key []byte) error {
	return b.db.Delete(ns, key)
}
