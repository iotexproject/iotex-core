// Copyright (c) 2024 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// AllKeys returns all keys in a bucket
func (b *BoltDBVersioned) AllKeys(ns string) (int, int, string, error) {
	var (
		total    int
		count    int
		nonDBErr bool
		h        = make([]byte, 0, 40000)
	)
	err := b.db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("Meta"))
		if bucket == nil {
			nonDBErr = true
			return errors.Wrap(ErrBucketNotExist, "metadata bucket doesn't exist")
		}
		height := bucket.Get([]byte("currentHeight"))
		if height == nil {
			return errors.Wrap(ErrNotExist, "height doesn't exist")
		}
		println("height =", byteutil.BytesToUint64(height))
		bucket = tx.Bucket([]byte(ns))
		if bucket == nil {
			nonDBErr = true
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", []byte(ns))
		}
		// get key length
		var (
			c          = bucket.Cursor()
			k, v       = c.First()
			vn         *versionedNamespace
			err        error
			keyMetaLen int
		)
		if v == nil {
			nonDBErr = true
			return errors.Wrapf(ErrInvalid, "failed to get metadata for bucket = %x", []byte(ns))
		}
		if vn, err = deserializeVersionedNamespace(v); err != nil {
			nonDBErr = true
			return errors.Wrapf(err, "failed to get metadata of bucket %s", ns)
		}
		keyMetaLen = int(vn.keyLen) + 1
		for ; k != nil; k, v = c.Next() {
			total++
			h = append(h, v...)
			if total%1000 == 0 {
				m := hash.Hash160b(h[:])
				h = h[:len(m)]
				copy(h, m[:])
			}
			if len(k) == keyMetaLen {
				count++
				// km, err := deserializeKeyMeta(v)
				// if km == nil {
				// 	return errors.Wrapf(ErrInvalid, "failed to get metadata for key = %x", k[:keyMetaLen-1])
				// }
				// if err != nil {
				// 	return errors.Wrapf(err, "failed to get metadata of key = %x", k[:keyMetaLen-1])
				// }
			}
		}
		return nil
	})
	if nonDBErr {
		return 0, 0, "", err
	}
	if err != nil {
		return 0, 0, "", errors.Wrap(ErrIO, err.Error())
	}
	m := hash.Hash160b(h[:])
	return total, count, hex.EncodeToString(m[:]), nil
}

// Purge removes key up to (including) the given version
func (b *BoltDBVersioned) Purge(version uint64, ns string) (map[string]int, error) {
	var (
		err      error
		nonDBErr bool
		total    int
		bailout  int
		nsb      = []byte(ns)
		count    = make(map[string]int)
	)
	start := time.Now()
	for c := uint8(0); c < b.db.config.NumRetries; c++ {
		var (
			entries         = 0
			purgeInProgress bool
		)
		if err = b.db.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(nsb)
			if bucket == nil {
				nonDBErr = true
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", nsb)
			}
			var (
				c          = bucket.Cursor()
				k, v       = c.First()
				vn         *versionedNamespace
				km         *keyMeta
				err        error
				keyMetaLen int
			)
			// check bucket metadata
			if v == nil {
				nonDBErr = true
				return errors.Wrapf(ErrInvalid, "failed to get metadata for bucket = %x", nsb)
			}
			if vn, err = deserializeVersionedNamespace(v); err != nil {
				nonDBErr = true
				return errors.Wrapf(err, "failed to get metadata of bucket %s", ns)
			}
			keyMetaLen = int(vn.keyLen) + 1
			// check if there's a purge in progress
			mBucket := tx.Bucket([]byte("Meta"))
			if mBucket == nil {
				nonDBErr = true
				return errors.Wrap(ErrBucketNotExist, "metadata bucket doesn't exist")
			}
			if d := mBucket.Get(nsb); d != nil {
				// d is the key of the last delete
				fmt.Printf("load key = %x\n", d)
				k, v = c.Seek(d)
				purgeInProgress = true
			}
			var (
				dVersion uint64
				noPurge  bool
				dKey     []byte
			)
			for ; k != nil; k, v = c.Next() {
				total++
				if purgeInProgress {
					purgeInProgress = false
					goto purge
				}
				if len(k) != keyMetaLen {
					continue
				}
				km, err = deserializeKeyMeta(v)
				if err != nil {
					nonDBErr = true
					return errors.Wrapf(err, "failed to get metadata of key = %x", k[:keyMetaLen-1])
				}
				// delete all keys <= version
				dVersion, noPurge = km.updatePurge(version)
				if noPurge {
					bailout++
					c.Next()
					total++
					continue
				}
				dKey = versionedKey(k[:keyMetaLen-1], dVersion)
				k, _ = c.Next()
				total++
			purge:
				for ; bytes.Compare(k, dKey) <= 0; k, _ = c.Next() {
					total++
					entries++
					count[string(k[:keyMetaLen-1])]++
					if err = c.Delete(); err != nil {
						return err
					}
					if entries == 64000 {
						// purge in batch of 64k entries
						println("bailout =", bailout)
						fmt.Printf("last delete key = %x\n", k)
						return mBucket.Put(nsb, k)
					}
				}
			}
			fmt.Println("total =", total)
			fmt.Println("bailout =", bailout)
			return nil
		}); err == nil || nonDBErr {
			break
		}
	}
	if nonDBErr {
		return nil, err
	}
	if err != nil {
		return nil, errors.Wrap(ErrIO, err.Error())
	}
	fmt.Println("Time elapsed:", time.Now().Sub(start))
	return count, nil
}

func (b *BoltDBVersioned) randomWrite(ns string, count map[string]int) (int, error) {
	var (
		err  error
		max  int
		v, _ = hex.DecodeString("08011201301a200000000000000000000000000000000000000000000000000000000000000000")
	)
	for c := uint8(0); c < b.db.config.NumRetries; c++ {
		if err = b.db.db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists([]byte(ns))
			if err != nil {
				return errors.Wrapf(err, "failed to create bucket %s", ns)
			}
			for key, n := range count {
				if n > max {
					max = n
				}
				k := []byte(key)
				for i := 0; i < n; i++ {
					if err = bucket.Put(versionedKey(k, 500000+uint64(i*10)), v); err != nil {
						return err
					}
				}
			}
			return nil
		}); err == nil {
			break
		}
	}
	return max, err
}
