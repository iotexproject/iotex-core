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
	VersionedDB interface {
		lifecycle.StartStopper

		// Put insert or update a record identified by (namespace, key)
		Put(uint64, string, []byte, []byte) error

		// Get gets a record by (namespace, key)
		Get(uint64, string, []byte) ([]byte, error)

		// Delete deletes a record by (namespace, key)
		Delete(uint64, string, []byte) error

		// CommitBatch writes a batch to the underlying DB
		CommitBatch(uint64, batch.KVStoreBatch) error

		// Filter returns <k, v> pair in a bucket that meet the condition
		Filter(uint64, string, Condition, []byte, []byte) ([][]byte, [][]byte, error)

		// Version returns the key's most recent version
		Version(string, []byte) (uint64, error)
	}

	// BoltDBVersioned is KvVersioned implementation based on bolt DB
	BoltDBVersioned struct {
		db  *BoltDB
		vns map[string]int // map of versioned namespace
	}

	// Namespace specifies the name and key length of the versioned namespace
	Namespace struct {
		ns     string
		keyLen uint32
	}
)

// BoltDBVersionedOption sets option for BoltDBVersioned
type BoltDBVersionedOption func(*BoltDBVersioned)

func VnsOption(ns ...Namespace) BoltDBVersionedOption {
	return func(k *BoltDBVersioned) {
		for _, v := range ns {
			k.vns[v.ns] = int(v.keyLen)
		}
	}
}

// NewBoltDBVersioned instantiates an BoltDB which implements VersionedDB
func NewBoltDBVersioned(cfg Config, opts ...BoltDBVersionedOption) *BoltDBVersioned {
	b := BoltDBVersioned{
		db:  NewBoltDB(cfg),
		vns: make(map[string]int),
	}
	for _, opt := range opts {
		opt(&b)
	}
	return &b
}

// Start starts the DB
func (b *BoltDBVersioned) Start(ctx context.Context) error {
	if err := b.db.Start(ctx); err != nil {
		return err
	}
	return b.addVersionedNamespace()
}

// Stop stops the DB
func (b *BoltDBVersioned) Stop(ctx context.Context) error {
	return b.db.Stop(ctx)
}

func (b *BoltDBVersioned) addVersionedNamespace() error {
	for ns, keyLen := range b.vns {
		vn, err := b.checkNamespace(ns)
		if cause := errors.Cause(err); cause == ErrNotExist || cause == ErrBucketNotExist {
			// create metadata for namespace
			if err = b.db.Put(ns, _minKey, (&versionedNamespace{
				keyLen: uint32(keyLen),
			}).serialize()); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if vn.keyLen != uint32(keyLen) {
			return errors.Wrapf(ErrInvalid, "namespace %s already exists with key length = %d, got %d", ns, vn.keyLen, keyLen)
		}
	}
	return nil
}

// Put writes a <key, value> record
func (b *BoltDBVersioned) Put(version uint64, ns string, key, value []byte) error {
	if !b.db.IsReady() {
		return ErrDBNotStarted
	}
	keyLen, ok := b.vns[ns]
	if !ok {
		return b.db.Put(ns, key, value)
	}
	// check key length
	if len(key) != keyLen {
		return errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", keyLen, len(key))
	}
	last, _, err := b.get(math.MaxUint64, ns, key)
	if !isNotExist(err) && version < last {
		// not allowed to perform write on an earlier version
		return errors.Wrapf(ErrInvalid, "cannot write at earlier version %d", version)
	}
	if version != last {
		return b.db.Put(ns, keyForWrite(key, version), value)
	}
	buf := batch.NewBatch()
	buf.Delete(ns, keyForDelete(key, version), fmt.Sprintf("failed to delete key %x", key))
	buf.Put(ns, keyForWrite(key, version), value, fmt.Sprintf("failed to put key %x", key))
	return b.db.WriteBatch(buf)
}

// Get retrieves the most recent version
func (b *BoltDBVersioned) Get(version uint64, ns string, key []byte) ([]byte, error) {
	if !b.db.IsReady() {
		return nil, ErrDBNotStarted
	}
	keyLen, ok := b.vns[ns]
	if !ok {
		return b.db.Get(ns, key)
	}
	// check key length
	if len(key) != keyLen {
		return nil, errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", keyLen, len(key))
	}
	_, v, err := b.get(version, ns, key)
	if errors.Cause(err) == ErrDeleted {
		err = errors.Wrapf(ErrNotExist, "key %x deleted", key)
	}
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
	keyLen, ok := b.vns[ns]
	if !ok {
		return b.db.Delete(ns, key)
	}
	// check key length
	if len(key) != keyLen {
		return errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", keyLen, len(key))
	}
	last, _, err := b.get(math.MaxUint64, ns, key)
	if isNotExist(err) {
		return nil
	}
	if version < last {
		// not allowed to perform delete on an earlier version
		return errors.Wrapf(ErrInvalid, "cannot delete at earlier version %d", version)
	}
	if version != last {
		return b.db.Put(ns, keyForDelete(key, version), nil)
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
	keyLen, ok := b.vns[ns]
	if !ok {
		return 0, errors.Errorf("namespace %s is non-versioned", ns)
	}
	// check key length
	if len(key) != keyLen {
		return 0, errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", keyLen, len(key))
	}
	last, _, err := b.get(math.MaxUint64, ns, key)
	if isNotExist(err) {
		// key not yet written
		err = errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
	}
	return last, err
}

// Filter returns <k, v> pair in a bucket that meet the condition
func (b *BoltDBVersioned) Filter(version uint64, ns string, cond Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	if _, ok := b.vns[ns]; ok {
		panic("Filter not supported for versioned DB")
	}
	return b.db.Filter(ns, cond, minKey, maxKey)
}

// CommitBatch write a batch to DB, where the batch can contain keys for
// both versioned and non-versioned namespace
func (b *BoltDBVersioned) CommitBatch(version uint64, kvsb batch.KVStoreBatch) error {
	ve, nve, err := dedup(b.vns, kvsb)
	if err != nil {
		return errors.Wrapf(err, "BoltDBVersioned failed to write batch")
	}
	return b.commitBatch(version, ve, nve)
}

func (b *BoltDBVersioned) commitBatch(version uint64, ve, nve []*batch.WriteInfo) error {
	var (
		err      error
		nonDBErr bool
	)
	for c := uint8(0); c < b.db.config.NumRetries; c++ {
		buckets := make(map[string]*bolt.Bucket)
		if err = b.db.db.Update(func(tx *bolt.Tx) error {
			// create/check metadata of all namespaces
			for ns := range b.vns {
				if _, ok := buckets[ns]; !ok {
					bucket, err := tx.CreateBucketIfNotExists([]byte(ns))
					if err != nil {
						return errors.Wrapf(err, "failed to create bucket %s", ns)
					}
					buckets[ns] = bucket
				}
			}
			// keep order of the writes same as the original batch
			for i := len(ve) - 1; i >= 0; i-- {
				var (
					write = ve[i]
					ns    = write.Namespace()
					key   = write.Key()
				)
				// get bucket
				bucket, ok := buckets[ns]
				if !ok {
					panic(fmt.Sprintf("BoltDBVersioned.commitToDB(), vns = %s does not exist", ns))
				}
				// wrong-size key should be caught in dedup(), but check anyway
				if b.vns[ns] != len(key) {
					panic(fmt.Sprintf("BoltDBVersioned.commitToDB(), expect vnsize[%s] = %d, got %d", ns, b.vns[ns], len(key)))
				}
				nonDBErr, err = writeVersionedEntry(version, bucket, write)
				if err != nil {
					return err
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

func writeVersionedEntry(version uint64, bucket *bolt.Bucket, ve *batch.WriteInfo) (bool, error) {
	var (
		key      = ve.Key()
		val      = ve.Value()
		last     uint64
		notexist bool
		maxKey   = keyForWrite(key, math.MaxUint64)
	)
	c := bucket.Cursor()
	k, _ := c.Seek(maxKey)
	if k == nil || bytes.Compare(k, maxKey) == 1 {
		k, _ = c.Prev()
		if k == nil || bytes.Compare(k, keyForDelete(key, 0)) <= 0 {
			// cursor is at the beginning/end of the bucket or smaller than minimum key
			notexist = true
		}
	}
	if !notexist {
		_, last = parseKey(k)
	}
	switch ve.WriteType() {
	case batch.Put:
		if !notexist && version <= last {
			// not allowed to perform write on an earlier version
			return true, errors.Wrapf(ErrInvalid, "cannot write at earlier version %d", version)
		}
		if err := bucket.Put(keyForWrite(key, version), val); err != nil {
			return false, errors.Wrap(err, ve.Error())
		}
	case batch.Delete:
		if notexist {
			return false, nil
		}
		if version < last {
			// not allowed to perform delete on an earlier version
			return true, errors.Wrapf(ErrInvalid, "cannot delete at earlier version %d", version)
		}
		if err := bucket.Put(keyForDelete(key, version), nil); err != nil {
			return false, errors.Wrap(err, ve.Error())
		}
		if version == last {
			if err := bucket.Delete(keyForWrite(key, version)); err != nil {
				return false, errors.Wrap(err, ve.Error())
			}
		}
	}
	return false, nil
}

// dedup does 3 things:
// 1. deduplicate entries in the batch, only keep the last write for each key
// 2. splits entries into 2 slices according to the input namespace map
// 3. return a map of input namespace's keyLength
func dedup(vns map[string]int, kvsb batch.KVStoreBatch) ([]*batch.WriteInfo, []*batch.WriteInfo, error) {
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
			return nil, nil, e
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
		if pickAll {
			if n, ok := nsKeyLen[k.ns]; !ok {
				nsKeyLen[k.ns] = len(write.Key())
			} else {
				if n != len(write.Key()) {
					return nil, nil, errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", n, len(write.Key()))
				}
			}
			nsInMap = append(nsInMap, write)
		} else if keyLen := vns[k.ns]; keyLen > 0 {
			// verify key size
			if keyLen != len(write.Key()) {
				return nil, nil, errors.Wrapf(ErrInvalid, "invalid key length, expecting %d, got %d", keyLen, len(write.Key()))
			}
			nsInMap = append(nsInMap, write)
		} else {
			other = append(other, write)
		}
	}
	return nsInMap, other, nil
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
	if err != nil {
		return nil, err
	}
	return deserializeVersionedNamespace(data)
}
