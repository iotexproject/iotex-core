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
	VersionedDB interface {
		lifecycle.StartStopper

		// Put insert or update a record identified by (namespace, key)
		Put(uint64, string, []byte, []byte) error

		// Get gets a record by (namespace, key)
		Get(uint64, string, []byte) ([]byte, error)

		// Delete deletes a record by (namespace, key)
		Delete(uint64, string, []byte) error

		// Base returns the underlying KVStore
		Base() KVStore

		// Version returns the key's most recent version
		Version(string, []byte) (uint64, error)
	}

	// BoltDBVersioned is KvVersioned implementation based on bolt DB
	BoltDBVersioned struct {
		db *BoltDB
	}
)

// NewBoltDBVersioned instantiates an BoltDB which implements VersionedDB
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
