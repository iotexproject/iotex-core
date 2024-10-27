// Copyright (c) 2023 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/crypto"
	"github.com/iotexproject/iotex-core/v2/db/versionpb"
)

// versionedNamespace is the metadata for versioned namespace
type versionedNamespace struct {
	keyLen uint32
}

// serialize to bytes
func (vn *versionedNamespace) serialize() []byte {
	return byteutil.Must(proto.Marshal(vn.toProto()))
}

func (vn *versionedNamespace) toProto() *versionpb.VersionedNamespace {
	return &versionpb.VersionedNamespace{
		KeyLen: vn.keyLen,
	}
}

func fromProtoVN(pb *versionpb.VersionedNamespace) *versionedNamespace {
	return &versionedNamespace{
		keyLen: pb.KeyLen,
	}
}

// deserializeVersionedNamespace deserializes byte-stream to VersionedNamespace
func deserializeVersionedNamespace(buf []byte) (*versionedNamespace, error) {
	var vn versionpb.VersionedNamespace
	if err := proto.Unmarshal(buf, &vn); err != nil {
		return nil, err
	}
	return fromProtoVN(&vn), nil
}

func (b *BoltDBVersioned) Buckets() ([]string, error) {
	var (
		ns  []string
		err error
	)
	if err = b.db.db.View(func(tx *bolt.Tx) error {
		// Iterate over all top-level buckets
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			ns = append(ns, string(name))
			return nil
		})
	}); err != nil {
		return nil, err
	}
	return ns, nil
}

func (b *BoltDBVersioned) Hash() (string, error) {
	ns, err := b.Buckets()
	if err != nil {
		return "", err
	}

	h := make([]hash.Hash256, len(ns))
	for i, bucket := range ns {
		h[i], err = b.bucketHash(bucket)
		if err != nil {
			return "", err
		}
		fmt.Printf("bucket = %s, hash = %x\n", bucket, h[i])
	}
	hash := crypto.NewMerkleTree(h).HashTree()
	return hex.EncodeToString(hash[:]), nil
}

func (b *BoltDBVersioned) bucketHash(ns string) (hash.Hash256, error) {
	var h hash.Hash256
	err := b.db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ns))
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket %s", ns)
		}
		var (
			c     = bucket.Cursor()
			bytes = make([]byte, 0, 32)
		)
		for k, v := c.First(); k != nil; k, v = c.Next() {
			bytes = append(bytes, k...)
			bytes = append(bytes, v...)
			h = hash.Hash256b(bytes)
			copy(bytes, h[:])
			bytes = bytes[:32]
		}
		return nil
	})
	return h, err
}

func (b *BoltDBVersioned) listBucket(ns string) error {
	return b.db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ns))
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket %s", ns)
		}
		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key = %x\n", k)
			fmt.Printf("val = %x\n", hash.Hash256b(v))
		}
		return nil
	})
}

func (b *BoltDBVersioned) countHeightKey(ns string, height uint64) (int, error) {
	var count int
	err := b.db.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(ns))
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket %s", ns)
		}
		c := bucket.Cursor()
		c.First()
		for k, _ := c.Next(); k != nil; k, _ = c.Next() {
			_, h := parseKey(k)
			if h == height {
				count++
			}
		}
		return nil
	})
	return count, err
}

func IsVersioned(ns string) bool {
	return ns == "Account" || ns == "Contract"
}

func (b *BoltDBVersioned) CopyBucket(ns string, srcDB *BoltDBVersioned) error {
	if IsVersioned(ns) {
		return errors.Wrapf(ErrInvalid, "ns = %s is versioned", ns)
	}
	if err := b.db.Delete(ns, nil); err != nil {
		return err
	}
	println("======= delete and copy bucket =", ns)
	var (
		err      error
		finished bool
		total    uint64
		start    = time.Now()
	)
	return srcDB.db.db.View(func(tx *bolt.Tx) error {
		srcBucket := tx.Bucket([]byte(ns))
		if srcBucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %s doesn't exist", ns)
		}
		c := srcBucket.Cursor()
		for !finished {
			entries := 0
			if err = b.db.db.Update(func(tx *bolt.Tx) error {
				bucket, err := tx.CreateBucketIfNotExists([]byte(ns))
				if err != nil || bucket == nil {
					return errors.Wrapf(ErrBucketNotExist, "bucket = %s doesn't exist", ns)
				}
				mBucket := tx.Bucket([]byte("Meta"))
				if mBucket == nil {
					return errors.Wrap(ErrBucketNotExist, "metadata bucket doesn't exist")
				}
				var k, v []byte
				if kLast := mBucket.Get([]byte(ns)); kLast != nil {
					// there's a transform in progress
					k, v = c.Seek(kLast)
					fmt.Printf("continue from key = %x\n", k)
				} else {
					k, v = c.First()
					fmt.Printf("first key = %x\n", k)
				}
				for ; k != nil; k, v = c.Next() {
					if err := bucket.Put(k, v); err != nil {
						return err
					}
					total++
					if total%1000000 == 0 {
						fmt.Printf("copy %d entries\n", total)
					}
					entries++
					if entries == 128000 {
						// commit the tx, and write the next key
						if k, _ = c.Next(); k != nil {
							fmt.Printf("next key = %x\n", k)
							return mBucket.Put([]byte(ns), k)
						} else {
							// hit the end of the bucket
							break
						}
					}
				}
				finished = true
				return mBucket.Delete([]byte(ns))
			}); err != nil {
				println("encounter error:", err.Error())
				break
			}
		}
		fmt.Printf("ns = %s, copy %d entries, time = %v\n", ns, total, time.Since(start))
		return err
	})
}

func (b *BoltDBVersioned) CopyVersionedBucket(ns string, version uint64, srcDB *BoltDBVersioned) error {
	var (
		err       error
		finished  bool
		keyLength int
	)
	// key length
	if ns == "Account" {
		keyLength = 20 + 9
	} else if ns == "Contract" {
		keyLength = 32 + 9
	} else {
		return errors.Wrapf(ErrInvalid, "ns = %s is not versioned", ns)
	}
	println("======= copy versioned bucket =", ns)
	var (
		total uint64
		skip  uint64
		start = time.Now()
	)
	return srcDB.db.db.View(func(tx *bolt.Tx) error {
		srcBucket := tx.Bucket([]byte(ns))
		if srcBucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %s doesn't exist", ns)
		}
		c := srcBucket.Cursor()
		for !finished {
			entries := 0
			if err = b.db.db.Update(func(tx *bolt.Tx) error {
				bucket, err := tx.CreateBucketIfNotExists([]byte(ns))
				if err != nil || bucket == nil {
					return errors.Wrapf(ErrBucketNotExist, "bucket = %s doesn't exist", ns)
				}
				mBucket := tx.Bucket([]byte("Meta"))
				if mBucket == nil {
					return errors.Wrap(ErrBucketNotExist, "metadata bucket doesn't exist")
				}
				var k, v []byte
				if kLast := mBucket.Get([]byte(ns)); kLast != nil {
					// there's a transform in progress
					k, v = c.Seek(kLast)
					fmt.Printf("continue from key = %x\n", k)
				} else {
					c.First()
					k, v = c.Next()
					fmt.Printf("first key = %x\n", k)
				}
				for ; k != nil; k, v = c.Next() {
					if len(k) != keyLength {
						fmt.Printf("key = %x, length = %d", k, len(k))
						return errors.Wrapf(ErrInvalid, "key length = %d, expecting %d", len(k), keyLength)
					}
					if _, height := parseKey(k); height == version {
						skip++
						continue
					}
					if err := bucket.Put(k, v); err != nil {
						return err
					}
					total++
					if total%1000000 == 0 {
						fmt.Printf("copy %d entries\n", total)
					}
					entries++
					if entries == 128000 {
						// commit the tx, and write the next key
						if k, _ = c.Next(); k != nil {
							fmt.Printf("next key = %x\n", k)
							return mBucket.Put([]byte(ns), k)
						} else {
							// hit the end of the bucket
							break
						}
					}
				}
				finished = true
				return mBucket.Delete([]byte(ns))
			}); err != nil {
				println("encounter error:", err.Error())
				break
			}
		}
		fmt.Printf("ns = %s, skip = %d, copy %d entries, time = %v\n", ns, skip, total, time.Since(start))
		return err
	})
}

func (b *BoltDBVersioned) TransformToVersioned(version uint64, ns string) error {
	var (
		err error
	)
	if ns == "Account" {
		if err = b.db.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(ns))
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", []byte(ns))
			}
			// move height key to meta namespace
			hKey := []byte("currentHeight")
			v := bucket.Get(hKey)
			if v != nil {
				mBucket, err := tx.CreateBucketIfNotExists([]byte("Meta"))
				if err != nil {
					return err
				}
				if err = mBucket.Put(hKey, v); err != nil {
					return err
				}
				if err = bucket.Delete(hKey); err != nil {
					return err
				}
				println("moved height key")
			}
			return nil
		}); err != nil {
			return err
		}
	}
	var (
		finished  bool
		keyLength int
	)
	// key length
	if ns == "Account" {
		keyLength = 20
	} else if ns == "Contract" {
		keyLength = 32
	} else {
		return errors.Wrapf(ErrInvalid, "ns = %s is not versioned", ns)
	}
	// convert keys to versioned
	var (
		total uint64
		start = time.Now()
	)
	for !finished {
		entries := 0
		if err = b.db.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(ns))
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", []byte(ns))
			}
			mBucket := tx.Bucket([]byte("Meta"))
			if mBucket == nil {
				return errors.Wrap(ErrBucketNotExist, "metadata bucket doesn't exist")
			}
			var (
				c    = bucket.Cursor()
				k, v []byte
			)
			if kLast := mBucket.Get([]byte(ns)); kLast != nil {
				// there's a transform in progress
				k, v = c.Seek(kLast)
				fmt.Printf("continue from key = %x\n", k)
			} else {
				k, v = c.First()
				fmt.Printf("first key = %x\n", k)
			}
			for ; k != nil; k, v = c.Next() {
				if len(k) != keyLength {
					fmt.Printf("key = %x, length = %d", k, len(k))
					return errors.Wrapf(ErrInvalid, "key length = %d, expecting %d", len(k), keyLength)
				}
				if err = bucket.Put(keyForWrite(k, version), v); err != nil {
					return err
				}
				if err = bucket.Delete(k); err != nil {
					return err
				}
				total++
				if total%1000000 == 0 {
					fmt.Printf("commit %d entries\n", total)
				}
				entries++
				if entries == 256000 {
					// commit the tx, and write the next key
					if k, _ = c.Next(); k != nil {
						fmt.Printf("next key = %x\n", k)
						return mBucket.Put([]byte(ns), k)
					} else {
						// hit the end of the bucket
						break
					}
				}
			}
			finished = true
			if err = mBucket.Delete([]byte(ns)); err != nil {
				return err
			}
			// finally write the namespace metadata
			vn := versionedNamespace{
				keyLen: uint32(keyLength),
			}
			return bucket.Put(_minKey, vn.serialize())
		}); err != nil {
			break
		}
	}
	fmt.Printf("commit %d entries, time = %v\n", total, time.Since(start))
	return err
}
