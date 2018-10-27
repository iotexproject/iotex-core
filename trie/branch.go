// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"
	"encoding/gob"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

func (b *branch) get(key []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	return getHelper(b, key, prefix, dao, bucket, cb)
}

func (b *branch) upsert(
	key, value []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	return upsertHelper(b, key, value, prefix, dao, bucket, cb)
}

func (b *branch) delete(key []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	return deleteHelper(b, key, prefix, dao, bucket, cb)
}

// child returns the node's child, and length of matching path
func (b *branch) child(key []byte, dao db.KVStore, bucket string, cb db.CachedBatch) (patricia, int, error) {
	if hash := b.Path[key[0]]; hash != nil {
		child, err := getPatricia(hash, dao, bucket, cb)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "failed to get child for key = %x", key)
		}
		return child, 1, nil
	}
	return nil, 0, errors.Wrapf(ErrInvalidPatricia, "branch does not have path = %d", key[0])
}

// ascend updates the key as a result of child node update
func (b *branch) ascend(key []byte, index byte) error {
	b.Path[index] = nil
	if key != nil {
		b.Path[index] = make([]byte, len(key))
		copy(b.Path[index], key)
	}
	return nil
}

// insert <k, v> at current patricia node
func (b *branch) insert(k, v []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	divK := k[prefix:]
	node := b.Path[divK[0]]
	if node != nil {
		return nil, errors.Wrapf(ErrInvalidPatricia, "branch already has path = %d", k[0])
	}
	// create a new leaf
	l := leaf{prefix + 1, make([]byte, len(k)), v}
	copy(l.Path, k)
	hashl, err := putPatriciaNew(&l, bucket, cb)
	if err != nil {
		return nil, err
	}
	logger.Debug().Hex("newL", hashl[:8]).Int("prefix", l.Ext).Hex("path", k[l.Ext:]).Msg("splitB")
	// update with new child
	if err := b.ascend(hashl, k[prefix]); err != nil {
		return nil, err
	}
	return putPatriciaNew(b, bucket, cb)
}

// increase returns the number of nodes (B, E, L) being added as a result of insert()
func (b *branch) increase(key []byte) (int, int, int) {
	return 0, 0, 1
}

// collapse checks whether the node can collapse with its child, returns the hash of the new node
func (b *branch) collapse(
	_ patricia, key []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	active := 2
	path := []byte{}
	if key == nil {
		active = 0
		br := byte(0)
		// count number of remaining path
		for i := 0; i < RADIX; i++ {
			if len(b.Path[i]) > 0 {
				active++
				key = b.Path[i]
				br = byte(i)
			}
		}
		path = []byte{br}
	}

	if active == 0 {
		// no active branch, can be deleted
		return nil, nil
	}
	if active == 1 && prefix > 0 {
		// check if this node can combine with child into one node
		child, err := getPatricia(key, dao, bucket, cb)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to collapse key = %x", key)
		}
		if _, ok := child.(*leaf); ok {
			// delete from DB
			delPatricia(child, bucket, cb)
			// merge with child
			if err := child.merge(path); err != nil {
				return nil, err
			}
			logger.Debug().Msg("merge with child")
			// put node (with updated hash) into DB
			return putPatriciaNew(child, bucket, cb)
		}
		// branch can collapse into an ext
		e := leaf{EXTLEAF, path, key}
		// put node (with updated hash) into DB
		logger.Debug().Msg("turn into leaf")
		return putPatriciaNew(&e, bucket, cb)
	}
	// put node (with updated hash) into DB
	return putPatriciaNew(b, bucket, cb)
}

func (b *branch) merge([]byte) error {
	return errors.Wrapf(ErrInvalidPatricia, "merge() should not be called on branch")
}

// set assigns v to the node
func (b *branch) set([]byte) error {
	return errors.Wrapf(ErrInvalidPatricia, "set() should not be called on branch")
}

// blob return the <k, v> stored in the node
func (b *branch) blob([]byte) ([]byte, error) {
	// branch node stores the hash to next patricia node
	return nil, errors.Wrap(ErrInvalidPatricia, "branch does not store value")
}

// hash return the hash of this node
func (b *branch) hash() hash.Hash32B {
	stream := []byte{}
	for i := 0; i < RADIX; i++ {
		stream = append(stream, b.Path[i]...)
	}
	return blake2b.Sum256(stream)
}

// serialize to bytes
func (b *branch) serialize() ([]byte, error) {
	var stream bytes.Buffer
	enc := gob.NewEncoder(&stream)
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	// first byte denotes the type of patricia: 1-branch, 0-ext/leaf
	return append([]byte{BRANCH}, stream.Bytes()...), nil
}

// deserialize to branch
func (b *branch) deserialize(stream []byte) error {
	// reset variable
	*b = branch{}
	dec := gob.NewDecoder(bytes.NewBuffer(stream[1:]))
	return dec.Decode(b)
}

func (b *branch) print() {
	for i := 0; i < RADIX; i++ {
		if len(b.Path[i]) > 0 {
			logger.Info().Int("k", i).Hex("v", b.Path[i]).Msg("branch")
		}
	}
}
