// Copyright (c) 2024 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"
	"context"
	"fmt"
	"math"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

var (
	ErrDeleted = errors.New("deleted in DB")
	_minKey    = []byte{0} // the minimum key, used to store namespace's metadata
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

		// Base returns the underlying KVStore
		Base() KVStore

		// Version returns the key's most recent version
		Version(string, []byte) (uint64, error)

		// SetVersion sets the version, and returns a KVStore to call Put()/Get()
		SetVersion(uint64) KVStore
	}

	// BoltDBVersioned is KvVersioned implementation based on bolt DB
	BoltDBVersioned struct {
		db *BoltDB
	}
)

// NewBoltDBVersioned instantiates an BoltDB which implements KvVersioned
func NewBoltDBVersioned(cfg Config) *BoltDBVersioned {
	b := BoltDBVersioned{
		db: NewBoltDB(cfg),
	}
	return &b
}

// Start starts the DB
func (b *BoltDBVersioned) Start(ctx context.Context) error {
	return b.db.Start(ctx)
}

// Stop stops the DB
func (b *BoltDBVersioned) Stop(ctx context.Context) error {
	return b.db.Stop(ctx)
}

// Base returns the underlying KVStore
func (b *BoltDBVersioned) Base() KVStore {
	return b.db
}

// Put writes a <key, value> record
func (b *BoltDBVersioned) Put(version uint64, ns string, key, value []byte) error {
	if !b.db.IsReady() {
		return ErrDBNotStarted
	}
	// check namespace
	vn, err := b.checkNamespace(ns)
	if err != nil {
		return err
	}
	buf := batch.NewBatch()
	if vn == nil {
		// namespace not yet created
		buf.Put(ns, _minKey, (&versionedNamespace{
			keyLen: uint32(len(key)),
		}).serialize(), "failed to create metadata")
	} else {
		if len(key) != int(vn.keyLen) {
			return errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", vn.keyLen, len(key))
		}
		last, _, err := b.get(math.MaxUint64, ns, key)
		if !isNotExist(err) && version < last {
			// not allowed to perform write on an earlier version
			return ErrInvalid
		}
		buf.Delete(ns, keyForDelete(key, version), fmt.Sprintf("failed to delete key %x", key))
	}
	buf.Put(ns, keyForWrite(key, version), value, fmt.Sprintf("failed to put key %x", key))
	return b.db.WriteBatch(buf)
}

// Get retrieves the most recent version
func (b *BoltDBVersioned) Get(version uint64, ns string, key []byte) ([]byte, error) {
	if !b.db.IsReady() {
		return nil, ErrDBNotStarted
	}
	// check key's metadata
	if err := b.checkNamespaceAndKey(ns, key); err != nil {
		return nil, err
	}
	_, v, err := b.get(version, ns, key)
	return v, err
}

func (b *BoltDBVersioned) get(version uint64, ns string, key []byte) (uint64, []byte, error) {
	var (
		last     uint64
		isDelete bool
		value    []byte
	)
	err := b.db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ns))
		if bucket == nil {
			return ErrBucketNotExist
		}
		var (
			c    = bucket.Cursor()
			min  = keyForDelete(key, 0)
			key  = keyForWrite(key, version)
			k, v = c.Seek(key)
		)
		if k == nil || bytes.Compare(k, key) == 1 {
			k, v = c.Prev()
			if k == nil || bytes.Compare(k, min) <= 0 {
				// cursor is at the beginning/end of the bucket or smaller than minimum key
				return ErrNotExist
			}
		}
		isDelete, last = parseKey(k)
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	if err != nil {
		return last, nil, err
	}
	if isDelete {
		return last, nil, ErrDeleted
	}
	return last, value, nil
}

// Delete deletes a record, if key does not exist, it returns nil
func (b *BoltDBVersioned) Delete(version uint64, ns string, key []byte) error {
	if !b.db.IsReady() {
		return ErrDBNotStarted
	}
	// check key's metadata
	if err := b.checkNamespaceAndKey(ns, key); err != nil {
		return err
	}
	last, _, err := b.get(math.MaxUint64, ns, key)
	if isNotExist(err) {
		return err
	}
	if version < last {
		// not allowed to perform delete on an earlier version
		return ErrInvalid
	}
	buf := batch.NewBatch()
	buf.Put(ns, keyForDelete(key, version), nil, fmt.Sprintf("failed to delete key %x", key))
	buf.Delete(ns, keyForWrite(key, version), fmt.Sprintf("failed to delete key %x", key))
	return b.db.WriteBatch(buf)
}

// Version returns the key's most recent version
func (b *BoltDBVersioned) Version(ns string, key []byte) (uint64, error) {
	if !b.db.IsReady() {
		return 0, ErrDBNotStarted
	}
	// check key's metadata
	if err := b.checkNamespaceAndKey(ns, key); err != nil {
		return 0, err
	}
	last, _, err := b.get(math.MaxUint64, ns, key)
	if isNotExist(err) {
		// key not yet written
		err = errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
	}
	return last, err
}

func isNotExist(err error) bool {
	return err == ErrNotExist || err == ErrBucketNotExist
}

func keyForWrite(key []byte, v uint64) []byte {
	k := make([]byte, len(key), len(key)+9)
	copy(k, key)
	k = append(k, byteutil.Uint64ToBytesBigEndian(v)...)
	return append(k, 1)
}

func keyForDelete(key []byte, v uint64) []byte {
	k := make([]byte, len(key), len(key)+9)
	copy(k, key)
	k = append(k, byteutil.Uint64ToBytesBigEndian(v)...)
	return append(k, 0)
}

func parseKey(key []byte) (bool, uint64) {
	size := len(key)
	return (key[size-1] == 0), byteutil.BytesToUint64BigEndian(key[size-9 : size-1])
}

func (b *BoltDBVersioned) checkNamespace(ns string) (*versionedNamespace, error) {
	data, err := b.db.Get(ns, _minKey)
	switch errors.Cause(err) {
	case nil:
		vn, err := deserializeVersionedNamespace(data)
		if err != nil {
			return nil, err
		}
		return vn, nil
	case ErrNotExist, ErrBucketNotExist:
		return nil, nil
	default:
		return nil, err
	}
}

func (b *BoltDBVersioned) checkNamespaceAndKey(ns string, key []byte) error {
	vn, err := b.checkNamespace(ns)
	if err != nil {
		return err
	}
	if vn == nil {
		return errors.Wrapf(ErrNotExist, "namespace = %x doesn't exist", ns)
	}
	if len(key) != int(vn.keyLen) {
		return errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", vn.keyLen, len(key))
	}
	return nil
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
	panic("Filter not supported for versioned DB")
}

// WriteBatch commits a batch
func (b *KvWithVersion) WriteBatch(kvsb batch.KVStoreBatch) error {
	// TODO: implement WriteBatch
	return nil
}
