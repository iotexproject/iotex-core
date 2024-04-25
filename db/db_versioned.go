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
	}
	// check key's metadata
	km, err := b.checkKey(ns, key)
	if err != nil {
		return err
	}
	km, exit := km.updateWrite(version, value)
	if exit {
		// not a valid write request
		return ErrInvalid
	}
	buf.Put(ns, append(key, 0), km.serialize(), fmt.Sprintf("failed to put key %x's metadata", key))
	buf.Put(ns, versionedKey(key, version), value, fmt.Sprintf("failed to put key %x", key))
	return b.db.WriteBatch(buf)
}

// Get retrieves the most recent version
func (b *BoltDBVersioned) Get(version uint64, ns string, key []byte) ([]byte, error) {
	if !b.db.IsReady() {
		return nil, ErrDBNotStarted
	}
	// check key's metadata
	km, err := b.checkNamespaceAndKey(ns, key)
	if err != nil {
		return nil, err
	}
	hitLast, err := km.checkRead(version)
	if err != nil {
		return nil, errors.Wrapf(err, "key = %x", key)
	}
	if hitLast {
		return km.lastWrite, nil
	}
	last, v, err := b.get(version, ns, key)
	if err != nil {
		return nil, err
	}
	hitLast, err = km.hitLastWrite(last, version)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (b *BoltDBVersioned) get(version uint64, ns string, key []byte) (uint64, []byte, error) {
	// construct the actual key = key + version (represented in 8-bytes)
	// and read from DB
	var (
		last  uint64
		meta  = append(key, 0)
		value []byte
	)
	key = versionedKey(key, version)
	err := b.db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ns))
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", []byte(ns))
		}
		c := bucket.Cursor()
		k, v := c.Seek(key)
		if k == nil || bytes.Compare(k, key) == 1 {
			k, v = c.Prev()
			if k == nil || bytes.Compare(k, meta) <= 0 {
				// cursor is at the beginning/end of the bucket or smaller than key's meta
				panic(fmt.Sprintf("BoltDBVersioned.get(), invalid key = %x", key))
			}
		}
		last = byteutil.BytesToUint64BigEndian(k[len(k)-8:])
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	if err == nil {
		return last, value, nil
	}
	if cause := errors.Cause(err); cause == ErrNotExist || cause == ErrBucketNotExist {
		return 0, nil, err
	}
	return 0, nil, errors.Wrap(ErrIO, err.Error())
}

// Delete deletes a record, if key does not exist, it returns nil
func (b *BoltDBVersioned) Delete(version uint64, ns string, key []byte) error {
	if !b.db.IsReady() {
		return ErrDBNotStarted
	}
	// check key's metadata
	km, err := b.checkNamespaceAndKey(ns, key)
	if err != nil {
		return err
	}
	if km == nil {
		return errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
	}
	if err = km.updateDelete(version); err != nil {
		return err
	}
	return b.db.Put(ns, append(key, 0), km.serialize())
}

// Version returns the key's most recent version
func (b *BoltDBVersioned) Version(ns string, key []byte) (uint64, error) {
	if !b.db.IsReady() {
		return 0, ErrDBNotStarted
	}
	// check key's metadata
	km, err := b.checkNamespaceAndKey(ns, key)
	if err != nil {
		return 0, err
	}
	if km == nil {
		// key not yet written
		return 0, errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
	}
	if lastDelete := km.lastDelete(); lastDelete > km.lastVersion || (lastDelete == km.lastVersion && km.isDeleteAfterWrite(lastDelete)) {
		// there's a delete-after-write
		err = errors.Wrapf(ErrDeleted, "key = %x already deleted", key)
	}
	return km.lastVersion, err
}

// SetVersion sets the version, and returns a KVStore to call Put()/Get()
func (b *BoltDBVersioned) SetVersion(v uint64) KVStore {
	return &KvWithVersion{
		db:      b,
		version: v,
	}
}

func versionedKey(key []byte, v uint64) []byte {
	k := make([]byte, len(key), len(key)+8)
	copy(k, key)
	return append(k, byteutil.Uint64ToBytesBigEndian(v)...)
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

func (b *BoltDBVersioned) checkKey(ns string, key []byte) (*keyMeta, error) {
	data, err := b.db.Get(ns, append(key, 0))
	switch errors.Cause(err) {
	case nil:
		km, err := deserializeKeyMeta(data)
		if err != nil {
			return nil, err
		}
		return km, nil
	case ErrNotExist, ErrBucketNotExist:
		return nil, nil
	default:
		return nil, err
	}
}

func (b *BoltDBVersioned) checkNamespaceAndKey(ns string, key []byte) (*keyMeta, error) {
	vn, err := b.checkNamespace(ns)
	if err != nil {
		return nil, err
	}
	if vn == nil {
		return nil, errors.Wrapf(ErrNotExist, "namespace = %x doesn't exist", ns)
	}
	if len(key) != int(vn.keyLen) {
		return nil, errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", vn.keyLen, len(key))
	}
	return b.checkKey(ns, key)
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
