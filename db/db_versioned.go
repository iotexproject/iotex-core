// Copyright (c) 2024 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
)

type (
	// KvVersioned is a versioned key-value store, where each key can (but doesn't
	// have to) have multiple versions of value (corresponding to different heights
	// in a blockchain)
	//
	// A namespace/bucket is considered versioned by default, user needs to use the
	// NonversionedNamespaceOption() to specify non-versioned namespaces at the time
	// of creating the versioned key-value store
	//
	// Versioning is achieved by using (key + 8-byte version) as the actual storage
	// key in the underlying DB. For buckets containing versioned keys, a metadata
	// is stored at the special key = []byte{0}. The metadata specifies the bucket's
	// name and the key length
	//
	// For each versioned key, the special location = key + []byte{0} stores the
	// key's metadata, which includes the following info:
	// 1. the version when the key is first created
	// 2. the version when the key is lastly written
	// 3. the version when the key is deleted
	// 4. hash of the key's last written value
	// If the location does not store a value, the key has never been written.
	//
	// Here's an example of a versioned DB which has 3 buckets:
	// 1. "mta" -- regular bucket storing metadata, key is not versioned
	// 2. "act" -- versioned namespace, key length = 20
	// 3. "stk" -- versioned namespace, key length = 8
	KvVersioned interface {
		KVStore

		// Version returns the key's most recent version
		Version(string, []byte) (uint64, error)

		// SetVersion sets the version before calling Put()
		SetVersion(uint64) KvVersioned

		// CreateVersionedNamespace creates a namespace to store versioned keys
		CreateVersionedNamespace(string, uint32) error
	}

	// BoltDBVersioned is KvVersioned implementation based on bolt DB
	BoltDBVersioned struct {
		*BoltDB
		version      uint64          // version for Get/Put()
		versioned    map[string]int  // buckets for versioned keys
		nonversioned map[string]bool // buckets for non-versioned keys
	}
)

// Option sets an option
type Option func(b *BoltDBVersioned)

// NonversionedNamespaceOption sets non-versioned namespace
func NonversionedNamespaceOption(ns ...string) Option {
	return func(b *BoltDBVersioned) {
		for _, v := range ns {
			b.nonversioned[v] = true
		}
	}
}

// VersionedNamespaceOption sets versioned namespace
func VersionedNamespaceOption(ns string, n int) Option {
	return func(b *BoltDBVersioned) {
		b.versioned[ns] = n
	}
}

// NewBoltDBVersioned instantiates an BoltDB with implements KvVersioned
func NewBoltDBVersioned(cfg Config, opts ...Option) *BoltDBVersioned {
	b := &BoltDBVersioned{
		BoltDB:       NewBoltDB(cfg),
		versioned:    make(map[string]int),
		nonversioned: make(map[string]bool),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Start starts the DB
func (b *BoltDBVersioned) Start(ctx context.Context) error {
	if err := b.BoltDB.Start(ctx); err != nil {
		return err
	}
	// TODO: check non-versioned and versioned namespace
	return nil
}

// Put writes a <key, value> record
func (b *BoltDBVersioned) Put(ns string, key, value []byte) error {
	if !b.IsReady() {
		return ErrDBNotStarted
	}
	// TODO: implement Put
	return nil
}

// Get retrieves the most recent version
func (b *BoltDBVersioned) Get(ns string, key []byte) ([]byte, error) {
	if !b.IsReady() {
		return nil, ErrDBNotStarted
	}
	// TODO: implement Get
	return nil, nil
}

// Delete deletes a record,if key is nil,this will delete the whole bucket
func (b *BoltDBVersioned) Delete(ns string, key []byte) error {
	if !b.IsReady() {
		return ErrDBNotStarted
	}
	// TODO: implement Delete
	return nil
}

// Version returns the key's most recent version
func (b *BoltDBVersioned) Version(ns string, key []byte) (uint64, error) {
	if !b.IsReady() {
		return 0, ErrDBNotStarted
	}
	// TODO: implement Version
	return 0, nil
}

// SetVersion sets the version which a Get()/Put() wants to access
func (b *BoltDBVersioned) SetVersion(v uint64) KvVersioned {
	b.version = v
	return b
}

// CreateVersionedNamespace creates a namespace to store versioned keys
func (b *BoltDBVersioned) CreateVersionedNamespace(ns string, n uint32) error {
	if !b.IsReady() {
		return ErrDBNotStarted
	}
	// TODO: implement CreateVersionedNamespace
	return nil
}
