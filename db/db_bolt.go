// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const fileMode = 0600

var (
	// ErrDBNotStarted represents the error when a db has not started
	ErrDBNotStarted = errors.New("db has not started")
)

// BoltDB is KVStore implementation based bolt DB
type BoltDB struct {
	db     *bolt.DB
	path   string
	config Config
}

// NewBoltDB instantiates an BoltDB with implements KVStore
func NewBoltDB(cfg Config) *BoltDB {
	return &BoltDB{
		db:     nil,
		path:   cfg.DbPath,
		config: cfg,
	}
}

// Start opens the BoltDB (creates new file if not existing yet)
func (b *BoltDB) Start(_ context.Context) error {
	db, err := bolt.Open(b.path, fileMode, nil)
	if err != nil {
		return errors.Wrap(ErrIO, err.Error())
	}
	b.db = db
	return nil
}

// Stop closes the BoltDB
func (b *BoltDB) Stop(_ context.Context) error {
	if b.db != nil {
		if err := b.db.Close(); err != nil {
			return errors.Wrap(ErrIO, err.Error())
		}
	}
	return nil
}

// Put inserts a <key, value> record
func (b *BoltDB) Put(namespace string, key, value []byte) (err error) {
	if b.db == nil {
		return ErrDBNotStarted
	}

	for c := uint8(0); c < b.config.NumRetries; c++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
			if err != nil {
				return err
			}
			return bucket.Put(key, value)
		}); err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// Get retrieves a record
func (b *BoltDB) Get(namespace string, key []byte) ([]byte, error) {
	if b.db == nil {
		return nil, ErrDBNotStarted
	}

	var value []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(namespace))
		if bucket == nil {
			return errors.Wrapf(ErrNotExist, "bucket = %x doesn't exist", []byte(namespace))
		}
		v := bucket.Get(key)
		if v == nil {
			return errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
		}
		value = make([]byte, len(v))
		// TODO: this is not an efficient way of passing the data
		copy(value, v)
		return nil
	})
	if err == nil {
		return value, nil
	}
	if errors.Cause(err) == ErrNotExist {
		return nil, err
	}
	return nil, errors.Wrap(ErrIO, err.Error())
}

// Filter returns <k, v> pair in a bucket that meet the condition
func (b *BoltDB) Filter(namespace string, cond Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	if b.db == nil {
		return nil, nil, ErrDBNotStarted
	}

	var fk, fv [][]byte
	if err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(namespace))
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", []byte(namespace))
		}

		var k, v []byte
		c := bucket.Cursor()
		if len(minKey) > 0 {
			k, v = c.Seek(minKey)
		} else {
			k, v = c.First()
		}

		if k == nil {
			return nil
		}

		checkMax := len(maxKey) > 0
		for ; k != nil; k, v = c.Next() {
			if checkMax && bytes.Compare(k, maxKey) == 1 {
				return nil
			}
			if cond(k, v) {
				key := make([]byte, len(k))
				copy(key, k)
				value := make([]byte, len(v))
				copy(value, v)
				fk = append(fk, key)
				fv = append(fv, value)
			}
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}

	if len(fk) == 0 {
		return nil, nil, errors.Wrap(ErrNotExist, "filter returns no match")
	}
	return fk, fv, nil
}

// Range retrieves values for a range of keys
func (b *BoltDB) Range(namespace string, key []byte, count uint64) ([][]byte, error) {
	if b.db == nil {
		return nil, ErrDBNotStarted
	}

	value := make([][]byte, count)
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(namespace))
		if bucket == nil {
			return errors.Wrapf(ErrNotExist, "bucket = %s doesn't exist", namespace)
		}
		// seek to start
		cur := bucket.Cursor()
		k, v := cur.Seek(key)
		if k == nil {
			return errors.Wrapf(ErrNotExist, "entry for key 0x%x doesn't exist", key)
		}
		// retrieve 'count' items
		for i := uint64(0); i < count; i++ {
			if k == nil {
				return errors.Wrapf(ErrNotExist, "entry for key 0x%x doesn't exist", k)
			}
			value[i] = make([]byte, len(v))
			copy(value[i], v)
			k, v = cur.Next()
		}
		return nil
	})
	if err == nil {
		return value, nil
	}
	if errors.Cause(err) == ErrNotExist {
		return nil, err
	}
	return nil, errors.Wrap(ErrIO, err.Error())
}

// GetBucketByPrefix retrieves all bucket those with const namespace prefix
func (b *BoltDB) GetBucketByPrefix(namespace []byte) ([][]byte, error) {
	if b.db == nil {
		return nil, ErrDBNotStarted
	}

	allKey := make([][]byte, 0)
	err := b.db.View(func(tx *bolt.Tx) error {
		if err := tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			if bytes.HasPrefix(name, namespace) && !bytes.Equal(name, namespace) {
				temp := make([]byte, len(name))
				copy(temp, name)
				allKey = append(allKey, temp)
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	})
	return allKey, err
}

// GetKeyByPrefix retrieves all keys those with const prefix
func (b *BoltDB) GetKeyByPrefix(namespace, prefix []byte) ([][]byte, error) {
	if b.db == nil {
		return nil, ErrDBNotStarted
	}

	allKey := make([][]byte, 0)
	err := b.db.View(func(tx *bolt.Tx) error {
		buck := tx.Bucket(namespace)
		if buck == nil {
			return ErrNotExist
		}
		c := buck.Cursor()
		for k, _ := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			temp := make([]byte, len(k))
			copy(temp, k)
			allKey = append(allKey, temp)
		}
		return nil
	})
	return allKey, err
}

// Delete deletes a record,if key is nil,this will delete the whole bucket
func (b *BoltDB) Delete(namespace string, key []byte) (err error) {
	if b.db == nil {
		return ErrDBNotStarted
	}

	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		if key == nil {
			err = b.db.Update(func(tx *bolt.Tx) error {
				if err := tx.DeleteBucket([]byte(namespace)); err != bolt.ErrBucketNotFound {
					return err
				}
				return nil
			})
		} else {
			err = b.db.Update(func(tx *bolt.Tx) error {
				bucket := tx.Bucket([]byte(namespace))
				if bucket == nil {
					return nil
				}
				return bucket.Delete(key)
			})
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// WriteBatch commits a batch
func (b *BoltDB) WriteBatch(kvsb batch.KVStoreBatch) (err error) {
	if b.db == nil {
		return ErrDBNotStarted
	}

	kvsb.Lock()
	defer kvsb.Unlock()

	for c := uint8(0); c < b.config.NumRetries; c++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			for i := 0; i < kvsb.Size(); i++ {
				write, e := kvsb.Entry(i)
				if e != nil {
					return e
				}
				ns := write.Namespace()
				errFmt := write.ErrorFormat()
				errArgs := write.ErrorArgs()
				switch write.WriteType() {
				case batch.Put:
					bucket, e := tx.CreateBucketIfNotExists([]byte(ns))
					if e != nil {
						return errors.Wrapf(e, errFmt, errArgs)
					}
					if p, ok := kvsb.CheckFillPercent(ns); ok {
						bucket.FillPercent = p
					}
					if e := bucket.Put(write.Key(), write.Value()); e != nil {
						return errors.Wrapf(e, errFmt, errArgs)
					}
				case batch.Delete:
					bucket := tx.Bucket([]byte(ns))
					if bucket == nil {
						continue
					}
					if e := bucket.Delete(write.Key()); e != nil {
						return errors.Wrapf(e, errFmt, errArgs)
					}
				}
			}
			return nil
		}); err == nil {
			break
		}
	}

	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// BucketExists returns true if bucket exists
func (b *BoltDB) BucketExists(namespace string) bool {
	if b.db == nil {
		log.L().Debug("db hasn't started, unable to check if bucket exists")
		return false
	}

	var exist bool
	_ = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(namespace))
		if bucket != nil {
			exist = true
		}
		return nil
	})
	return exist
}

// ======================================
// below functions used by RangeIndex
// ======================================

// Insert inserts a value into the index
func (b *BoltDB) Insert(name []byte, key uint64, value []byte) error {
	if b.db == nil {
		return ErrDBNotStarted
	}

	var err error
	for i := uint8(0); i < b.config.NumRetries; i++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(name)
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", name)
			}
			cur := bucket.Cursor()
			ak := byteutil.Uint64ToBytesBigEndian(key - 1)
			k, v := cur.Seek(ak)
			if !bytes.Equal(k, ak) {
				// insert new key
				if err := bucket.Put(ak, v); err != nil {
					return err
				}
			} else {
				// update an existing key
				k, _ = cur.Next()
			}
			if k != nil {
				return bucket.Put(k, value)
			}
			return nil
		}); err == nil {
			break
		}
	}
	if err != nil {
		return errors.Wrap(ErrIO, err.Error())
	}
	return nil
}

// SeekNext returns value by the key (if key not exist, use next key)
func (b *BoltDB) SeekNext(name []byte, key uint64) ([]byte, error) {
	if b.db == nil {
		return nil, ErrDBNotStarted
	}

	var value []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(name)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", name)
		}
		// seek to start
		cur := bucket.Cursor()
		_, v := cur.Seek(byteutil.Uint64ToBytesBigEndian(key))
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// SeekPrev returns value by the key (if key not exist, use previous key)
func (b *BoltDB) SeekPrev(name []byte, key uint64) ([]byte, error) {
	if b.db == nil {
		return nil, ErrDBNotStarted
	}

	var value []byte
	if err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(name)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", name)
		}
		// seek to start
		cur := bucket.Cursor()
		cur.Seek(byteutil.Uint64ToBytesBigEndian(key))
		_, v := cur.Prev()
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	}); err != nil {
		return nil, err
	}
	return value, nil
}

// Remove removes an existing key
func (b *BoltDB) Remove(name []byte, key uint64) error {
	if b.db == nil {
		return ErrDBNotStarted
	}

	var err error
	for i := uint8(0); i < b.config.NumRetries; i++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(name)
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", name)
			}
			cur := bucket.Cursor()
			ak := byteutil.Uint64ToBytesBigEndian(key - 1)
			k, v := cur.Seek(ak)
			if !bytes.Equal(k, ak) {
				// return nil if the key does not exist
				return nil
			}
			if err := bucket.Delete(ak); err != nil {
				return err
			}
			// write the corresponding value to next key
			k, _ = cur.Next()
			if k != nil {
				return bucket.Put(k, v)
			}
			return nil
		}); err == nil {
			break
		}
	}
	return err
}

// Purge deletes an existing key and all keys before it
func (b *BoltDB) Purge(name []byte, key uint64) error {
	if b.db == nil {
		return ErrDBNotStarted
	}

	var err error
	for i := uint8(0); i < b.config.NumRetries; i++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(name)
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", name)
			}
			cur := bucket.Cursor()
			nk, _ := cur.Seek(byteutil.Uint64ToBytesBigEndian(key))
			// delete all keys before this key
			for k, _ := cur.Prev(); k != nil; k, _ = cur.Prev() {
				if err := bucket.Delete(k); err != nil {
					return err
				}
			}
			// write not exist value to next key
			if nk != nil {
				return bucket.Put(nk, NotExist)
			}
			return nil
		}); err == nil {
			break
		}
	}
	return err
}

// ======================================
// private functions
// ======================================

// intentionally fail to test DB can successfully rollback
func (b *BoltDB) batchPutForceFail(namespace string, key [][]byte, value [][]byte) error {
	if b.db == nil {
		return ErrDBNotStarted
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
		if err != nil {
			return err
		}
		if len(key) != len(value) {
			return errors.Wrap(ErrIO, "batch put <k, v> size not match")
		}
		for i := 0; i < len(key); i++ {
			if err := bucket.Put(key[i], value[i]); err != nil {
				return err
			}
			// intentionally fail to test DB can successfully rollback
			if i == len(key)-1 {
				return errors.Wrapf(ErrIO, "force fail to test DB rollback")
			}
		}
		return nil
	})
}
