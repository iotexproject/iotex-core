// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

const (
	// RADIX equals 2^Nibble, where Nibble is the number of bits to be parsed each time
	RADIX = 256
	// BRANCH means the node is a branch
	BRANCH = 1
	// EXTLEAF means the node is an ext or leaf
	EXTLEAF = 0
)

var (
	// ErrInvalidPatricia indicates invalid operation
	ErrInvalidPatricia = errors.New("invalid patricia operation")

	// ErrPathDiverge indicates the path diverges
	ErrPathDiverge = errors.New("path diverges")
)

type (
	patricia interface {
		get([]byte, int, db.KVStore, string, db.CachedBatch) ([]byte, error)
		upsert([]byte, []byte, int, db.KVStore, string, db.CachedBatch) ([]byte, error)
		delete([]byte, int, db.KVStore, string, db.CachedBatch) ([]byte, error)
		child([]byte, db.KVStore, string, db.CachedBatch) (patricia, int, error)
		ascend([]byte, byte) error
		insert([]byte, []byte, int, db.KVStore, string, db.CachedBatch) ([]byte, error)
		collapse(patricia, []byte, int, db.KVStore, string, db.CachedBatch) ([]byte, error)
		merge([]byte) error
		set([]byte) error
		blob([]byte) ([]byte, error)
		hash() hash.Hash32B // hash of this node
		serialize() ([]byte, error)
		deserialize([]byte) error
	}
	// key of next patricia node
	ptrcKey []byte
	// branch is the full node storing 256 hashes of next level node
	branch struct {
		Path [RADIX]ptrcKey
	}
	// there are 2 types of nodes
	// Leaf is always the final node on the path, storing actual value
	// Ext stores the squashed path, and hash of next node
	// for Leaf:
	// l.Ext  = the length of prefix path leading to this leaf
	// l.Path = the full path
	// l.Vaue = the actual value
	// for Ext:
	// e.Ext  = 0
	// e.Path = squashed path from its parent child leading to its child node
	// e.Vaue = hash of child node
	leaf struct {
		Ext   int
		Path  ptrcKey
		Value []byte
	}
)

// wrapper func for get()
// it continues to parse the key and calls child's get() recursively
func getHelper(
	node patricia, key []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	// parse the key and get child node
	child, match, err := node.child(key[prefix:], dao, bucket, cb)
	if err != nil {
		return nil, errors.Wrapf(ErrNotExist, "key = %x", key)
	}
	if child == nil {
		// this is the last node on path, return its stored value
		return node.blob(key)
	}
	// continue get() on child node
	return getHelper(child, key, prefix+match, dao, bucket, cb)
}

// wrapper func for upsert()
// it continues to parse the key and calls child's upsert() recursively
func upsertHelper(
	node patricia, key, value []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	// delete node from DB
	delPatricia(node, bucket, cb)
	// parse the key and get child node
	child, match, err := node.child(key[prefix:], dao, bucket, cb)
	if err != nil {
		// <k, v> does not exist, insert it
		return node.insert(key, value, prefix, dao, bucket, cb)
	}
	if child == nil {
		// this is the last node on path, update it
		if err := node.set(value); err != nil {
			return nil, errors.Wrapf(err, "failed to update value for key = %x", key)
		}
		// put into DB
		return putPatriciaNew(node, bucket, cb)
	}
	// continue upsert() on child node
	hash, err := upsertHelper(child, key, value, prefix+match, dao, bucket, cb)
	if err != nil {
		return nil, err
	}
	// update with child's new child
	if err := node.ascend(hash, key[prefix]); err != nil {
		return nil, err
	}
	// put into DB, hash of node changes as a result of upsert(), and is expected NOT to exist in DB
	return putPatriciaNew(node, bucket, cb)
}

// wrapper func for delete()
// it continues to parse the key and calls child's delete() recursively
func deleteHelper(
	node patricia, key []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	// delete node from DB
	delPatricia(node, bucket, cb)
	// parse the key and get child node
	child, match, err := node.child(key[prefix:], dao, bucket, cb)
	if err != nil {
		return nil, errors.Wrapf(ErrNotExist, "key = %x", key)
	}
	if child == nil {
		// this is the last node on path, return nil so parent can delete itself if possible
		return nil, nil
	}
	// continue delete() on child node
	hash, err := deleteHelper(child, key, prefix+match, dao, bucket, cb)
	if err != nil {
		return nil, err
	}
	// update with child's new hash
	if err := node.ascend(hash, key[prefix]); err != nil {
		return nil, err
	}
	// check if the node can collapse and combine with child
	return node.collapse(child, hash, prefix, dao, bucket, cb)
}

//======================================
// helper functions to operate patricia
//======================================
// getPatricia retrieves the patricia node from DB according to key
func getPatricia(key []byte, dao db.KVStore, bucket string, cb db.CachedBatch) (patricia, error) {
	// search in cache first
	node, err := cb.Get(bucket, key)
	if err != nil {
		node, err = dao.Get(bucket, key)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get key %x", key[:8])
	}
	var ptr patricia
	// first byte of serialized data is type
	switch node[0] {
	case BRANCH:
		ptr = &branch{}
	case EXTLEAF:
		ptr = &leaf{}
	default:
		return nil, errors.Wrapf(ErrInvalidPatricia, "invalid node type = %v", node[0])
	}
	if err := ptr.deserialize(node); err != nil {
		return nil, err
	}
	return ptr, nil
}

// putPatricia stores the patricia node into DB
// the node may already exist in DB
func putPatricia(ptr patricia, bucket string, cb db.CachedBatch) ([]byte, error) {
	value, err := ptr.serialize()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode patricia node")
	}
	key := ptr.hash()
	logger.Debug().Hex("key", key[:8]).Msg("put")
	cb.Put(bucket, key[:], value, "failed to put key = %x", key)
	return key[:], nil
}

// putPatriciaNew stores a new patricia node into DB
// it is expected the node does not exist yet, will return error if already exist
func putPatriciaNew(ptr patricia, bucket string, cb db.CachedBatch) ([]byte, error) {
	value, err := ptr.serialize()
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode patricia node")
	}
	key := ptr.hash()
	logger.Debug().Hex("key", key[:8]).Msg("putnew")
	return key[:], cb.PutIfNotExists(bucket, key[:], value, "failed to put non-existing key = %x", key)
}

// delPatricia deletes the patricia node from DB
func delPatricia(ptr patricia, bucket string, cb db.CachedBatch) {
	key := ptr.hash()
	logger.Debug().Hex("key", key[:8]).Msg("del")
	cb.Delete(bucket, key[:], "failed to delete key = %x", key)
}
