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
	"syscall"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
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
	}
	// check key's metadata
	km, err := b.checkKey(ns, key)
	if err != nil {
		return err
	}
	km, exit := km.updateWrite(version, value)
	if exit {
		return nil
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
	hitLast, err := km.updateRead(version)
	if err != nil {
		return nil, errors.Wrapf(err, "key = %x", key)
	}
	if hitLast {
		return km.lastWrite, nil
	}
	return b.get(version, ns, key)
}

func (b *BoltDBVersioned) get(version uint64, ns string, key []byte) ([]byte, error) {
	// construct the actual key = key + version (represented in 8-bytes)
	// and read from DB
	key = versionedKey(key, version)
	var value []byte
	err := b.db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ns))
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", []byte(ns))
		}
		c := bucket.Cursor()
		k, v := c.Seek(key)
		if k == nil || bytes.Compare(k, key) == 1 {
			k, v = c.Prev()
			if k == nil || bytes.Compare(k, append(key[:len(key)-8], 0)) <= 0 {
				// cursor is at the beginning/end of the bucket or smaller than minimum key
				panic(fmt.Sprintf("BoltDBVersioned.get(), invalid key = %x", key))
			}
		}
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	if err == nil {
		return value, nil
	}
	if cause := errors.Cause(err); cause == ErrNotExist || cause == ErrBucketNotExist {
		return nil, err
	}
	return nil, errors.Wrap(ErrIO, err.Error())
}

// Delete deletes a record, if key does not exist, it returns nil
func (b *BoltDBVersioned) Delete(version uint64, ns string, key []byte) error {
	if !b.db.IsReady() {
		return ErrDBNotStarted
	}
	// check key's metadata
	km, err := b.checkNamespaceAndKey(ns, key)
	if err != nil {
		if cause := errors.Cause(err); cause != ErrNotExist && cause != ErrInvalid {
			return err
		}
	}
	if km == nil || version < km.lastVersion || version <= km.deleteVersion {
		return nil
	}
	km.deleteVersion = version
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
	if km.deleteVersion != 0 {
		err = errors.Wrapf(ErrDeleted, "key = %x already deleted", key)
	}
	return km.lastVersion, err
}

// CommitToDB write a batch to DB, where the batch can contain keys for
// both versioned and non-versioned namespace
func (b *BoltDBVersioned) CommitToDB(version uint64, vns map[string]bool, kvsb batch.KVStoreBatch) error {
	vnsize, ve, nve, err := dedup(vns, kvsb)
	if err != nil {
		return errors.Wrapf(err, "BoltDBVersioned failed to write batch")
	}
	return b.commitToDB(version, vnsize, ve, nve)
}

func (b *BoltDBVersioned) commitToDB(version uint64, vnsize map[string]int, ve, nve []*batch.WriteInfo) error {
	var (
		err      error
		nonDBErr bool
	)
	for c := uint8(0); c < b.db.config.NumRetries; c++ {
		buckets := make(map[string]*bolt.Bucket)
		if err = b.db.db.Update(func(tx *bolt.Tx) error {
			// create/check metadata of all namespaces
			for ns, size := range vnsize {
				bucket, ok := buckets[ns]
				if !ok {
					bucket, err = tx.CreateBucketIfNotExists([]byte(ns))
					if err != nil {
						return errors.Wrapf(err, "failed to create bucket %s", ns)
					}
					buckets[ns] = bucket
				}
				var vn *versionedNamespace
				if val := bucket.Get(_minKey); val == nil {
					// namespace not created yet
					vn = &versionedNamespace{
						keyLen: uint32(size),
					}
					ve = append(ve, batch.NewWriteInfo(
						batch.Put, ns, _minKey, vn.serialize(),
						fmt.Sprintf("failed to create metadata for namespace %s", ns),
					))
				} else {
					if vn, err = deserializeVersionedNamespace(val); err != nil {
						nonDBErr = true
						return errors.Wrapf(err, "failed to get metadata of bucket %s", ns)
					}
					if vn.keyLen != uint32(size) {
						nonDBErr = true
						return errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", vn.keyLen, size)
					}
				}
			}
			// keep order of the writes same as the original batch
			for i := len(ve) - 1; i >= 0; i-- {
				var (
					write = ve[i]
					ns    = write.Namespace()
					key   = write.Key()
					km    *keyMeta
				)
				// get bucket
				bucket, ok := buckets[ns]
				if !ok {
					bucket, err = tx.CreateBucketIfNotExists([]byte(ns))
					if err != nil {
						return errors.Wrapf(err, "failed to create bucket %s", ns)
					}
					buckets[ns] = bucket
				}
				// check key metadata
				if val := bucket.Get(append(key, 0)); val != nil {
					if km, err = deserializeKeyMeta(val); err != nil {
						nonDBErr = true
						return errors.Wrapf(err, "failed to get metadata of key %x", key)
					}
				}
				switch write.WriteType() {
				case batch.Put:
					if bytes.Equal(key, _minKey) {
						// create namespace
						if err = bucket.Put(key, write.Value()); err != nil {
							return errors.Wrap(err, write.Error())
						}
					} else {
						// wrong-size key should be caught in dedup(), but check anyway
						if vnsize[ns] != len(key) {
							panic(fmt.Sprintf("BoltDBVersioned.commitToDB(), expect vnsize[%s] = %d, got %d", ns, vnsize[ns], len(key)))
						}
						km, exit := km.updateWrite(version, write.Value())
						if exit {
							// not a valid write
							continue
						}
						if err = bucket.Put(append(key, 0), km.serialize()); err != nil {
							return errors.Wrap(err, write.Error())
						}
						if err = bucket.Put(versionedKey(key, version), write.Value()); err != nil {
							return errors.Wrap(err, write.Error())
						}
					}
				case batch.Delete:
					if km == nil || version < km.lastVersion || version <= km.deleteVersion {
						continue
					}
					// wrong-size key should be caught in dedup(), but check anyway
					if vnsize[ns] != len(key) {
						panic(fmt.Sprintf("BoltDBVersioned.commitToDB(), expect vnsize[%s] = %d, got %d", ns, vnsize[ns], len(key)))
					}
					// mark the delete version
					km.deleteVersion = version
					if err = bucket.Put(append(key, 0), km.serialize()); err != nil {
						return errors.Wrap(err, write.Error())
					}
				}
			}
			// write non-versioned keys
			for i := len(nve) - 1; i >= 0; i-- {
				var (
					write = nve[i]
					ns    = write.Namespace()
				)
				switch write.WriteType() {
				case batch.Put:
					// get bucket
					bucket, ok := buckets[ns]
					if !ok {
						bucket, err = tx.CreateBucketIfNotExists([]byte(ns))
						if err != nil {
							return errors.Wrapf(err, "failed to create bucket %s", ns)
						}
						buckets[ns] = bucket
					}
					if err = bucket.Put(write.Key(), write.Value()); err != nil {
						return errors.Wrap(err, write.Error())
					}
				case batch.Delete:
					bucket := tx.Bucket([]byte(ns))
					if bucket == nil {
						continue
					}
					if err = bucket.Delete(write.Key()); err != nil {
						return errors.Wrap(err, write.Error())
					}
				}
			}
			return nil
		}); err == nil || nonDBErr {
			break
		}
	}
	if nonDBErr {
		return err
	}
	if err != nil {
		if errors.Is(err, syscall.ENOSPC) {
			log.L().Fatal("BoltDBVersioned failed to write batch", zap.Error(err))
		}
		return errors.Wrap(ErrIO, err.Error())
	}
	return nil
}

// dedup does 3 things:
// 1. deduplicate entries in the batch, only keep the last write for each key
// 2. splits entries into 2 slices according to the input namespace map
// 3. return a map of input namespace's keyLength
func dedup(vns map[string]bool, kvsb batch.KVStoreBatch) (map[string]int, []*batch.WriteInfo, []*batch.WriteInfo, error) {
	kvsb.Lock()
	defer kvsb.Unlock()

	type doubleKey struct {
		ns  string
		key string
	}

	var (
		entryKeySet = make(map[doubleKey]struct{})
		nsKeyLen    = make(map[string]int)
		nsInMap     = make([]*batch.WriteInfo, 0)
		other       = make([]*batch.WriteInfo, 0)
		pickAll     = len(vns) == 0
	)
	for i := kvsb.Size() - 1; i >= 0; i-- {
		write, e := kvsb.Entry(i)
		if e != nil {
			return nil, nil, nil, e
		}
		// only handle Put and Delete
		var (
			writeType = write.WriteType()
			ns        = write.Namespace()
			key       = write.Key()
		)
		if writeType != batch.Put && writeType != batch.Delete {
			continue
		}
		k := doubleKey{ns: ns, key: string(key)}
		if _, ok := entryKeySet[k]; !ok {
			if writeType == batch.Put {
				// for a DELETE, we want to capture the corresponding PUT
				entryKeySet[k] = struct{}{}
			}
			if pickAll || vns[k.ns] {
				nsInMap = append(nsInMap, write)
			} else {
				other = append(other, write)
			}
			// check key size
			if pickAll || vns[k.ns] {
				if n, ok := nsKeyLen[k.ns]; !ok {
					nsKeyLen[k.ns] = len(write.Key())
				} else {
					if n != len(write.Key()) {
						return nil, nil, nil, errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", n, len(write.Key()))
					}
				}
			}
		}
	}
	return nsKeyLen, nsInMap, other, nil
}

func versionedKey(key []byte, v uint64) []byte {
	return append(key, byteutil.Uint64ToBytesBigEndian(v)...)
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
		return nil, errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
	}
	if len(key) != int(vn.keyLen) {
		return nil, errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", vn.keyLen, len(key))
	}
	return b.checkKey(ns, key)
}

// Option sets an option
type Option func(*KvWithVersion)

func VersionedNamespaceOption(ns ...string) Option {
	return func(k *KvWithVersion) {
		k.versioned = make(map[string]bool)
		for _, ns := range ns {
			k.versioned[ns] = true
		}
	}
}

// KvWithVersion wraps the BoltDBVersioned with a certain version
type KvWithVersion struct {
	db        *BoltDBVersioned
	kvBase    KVStore
	versioned map[string]bool // map of versioned buckets
	version   uint64          // the current version
}

// NewKVStoreWithVersion implements a KVStore that can handle both versioned
// and non-versioned namespace
func NewKVStoreWithVersion(cfg Config, opts ...Option) *KvWithVersion {
	db := NewBoltDBVersioned(cfg)
	kv := KvWithVersion{
		db:     db,
		kvBase: db.Base(),
	}
	for _, opt := range opts {
		opt(&kv)
	}
	return &kv
}

// Start starts the DB
func (b *KvWithVersion) Start(ctx context.Context) error {
	return b.kvBase.Start(ctx)
}

// Stop stops the DB
func (b *KvWithVersion) Stop(ctx context.Context) error {
	return b.kvBase.Stop(ctx)
}

// Base returns the underlying KVStore
func (b *KvWithVersion) Base() KVStore {
	return b.kvBase
}

// Put writes a <key, value> record
func (b *KvWithVersion) Put(ns string, key, value []byte) error {
	if b.versioned[ns] {
		return b.db.Put(b.version, ns, key, value)
	}
	return b.kvBase.Put(ns, key, value)
}

// Get retrieves a key's value
func (b *KvWithVersion) Get(ns string, key []byte) ([]byte, error) {
	if b.versioned[ns] {
		return b.db.Get(b.version, ns, key)
	}
	return b.kvBase.Get(ns, key)
}

// Delete deletes a key
func (b *KvWithVersion) Delete(ns string, key []byte) error {
	if b.versioned[ns] {
		return b.db.Delete(b.version, ns, key)
	}
	return b.kvBase.Delete(ns, key)
}

// Filter returns <k, v> pair in a bucket that meet the condition
func (b *KvWithVersion) Filter(ns string, cond Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	if b.versioned[ns] {
		panic("Filter not supported for versioned DB")
	}
	return b.kvBase.Filter(ns, cond, minKey, maxKey)
}

// WriteBatch commits a batch
func (b *KvWithVersion) WriteBatch(kvsb batch.KVStoreBatch) error {
	return b.db.CommitToDB(b.version, b.versioned, kvsb)
}

// Version returns the key's most recent version
func (b *KvWithVersion) Version(ns string, key []byte) (uint64, error) {
	if b.versioned[ns] {
		return b.db.Version(ns, key)
	}
	return 0, errors.Errorf("namespace %s is non-versioned", ns)
}

// SetVersion sets the version, and returns a KVStore to call Put()/Get()
func (b *KvWithVersion) SetVersion(v uint64) KVStore {
	kv := KvWithVersion{
		db:        b.db,
		kvBase:    b.kvBase,
		versioned: make(map[string]bool),
		version:   v,
	}
	for k := range b.versioned {
		kv.versioned[k] = true
	}
	return &kv
}
