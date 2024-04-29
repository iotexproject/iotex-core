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
	"syscall"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
					val   = write.Value()
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
				// check key's last version
				var (
					last               uint64
					notexist, isDelete bool
					actualKey          = keyForWrite(key, version)
				)
				c := bucket.Cursor()
				k, _ := c.Seek(actualKey)
				if k == nil || bytes.Compare(k, actualKey) == 1 {
					k, _ = c.Prev()
					if k == nil || bytes.Compare(k, keyForDelete(key, 0)) <= 0 {
						// cursor is at the beginning/end of the bucket or smaller than minimum key
						notexist = true
					}
				}
				if !notexist {
					isDelete, last = parseKey(k)
				}
				switch write.WriteType() {
				case batch.Put:
					if bytes.Equal(key, _minKey) {
						// create namespace
						if err = bucket.Put(key, val); err != nil {
							return errors.Wrap(err, write.Error())
						}
					} else {
						// wrong-size key should be caught in dedup(), but check anyway
						if vnsize[ns] != len(key) {
							panic(fmt.Sprintf("BoltDBVersioned.commitToDB(), expect vnsize[%s] = %d, got %d", ns, vnsize[ns], len(key)))
						}
						if isDelete && version <= last {
							// not allowed to perform write on an earlier version
							nonDBErr = true
							return ErrInvalid
						}
						if err = bucket.Put(keyForWrite(key, version), val); err != nil {
							return errors.Wrap(err, write.Error())
						}
					}
				case batch.Delete:
					if notexist {
						continue
					}
					// wrong-size key should be caught in dedup(), but check anyway
					if vnsize[ns] != len(key) {
						panic(fmt.Sprintf("BoltDBVersioned.commitToDB(), expect vnsize[%s] = %d, got %d", ns, vnsize[ns], len(key)))
					}
					if version < last {
						// not allowed to perform delete on an earlier version
						nonDBErr = true
						return ErrInvalid
					}
					if err = bucket.Put(keyForDelete(key, version), nil); err != nil {
						return errors.Wrap(err, write.Error())
					}
					if err = bucket.Delete(keyForWrite(key, version)); err != nil {
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
		entryKeySet = make(map[doubleKey]bool)
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
		if entryKeySet[k] {
			continue
		}
		if writeType == batch.Put {
			// for a later DELETE, we want to capture the earlier PUT
			// otherwise, the DELETE might return not-exist
			entryKeySet[k] = true
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
	return nsKeyLen, nsInMap, other, nil
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
