// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
)

func (l *leaf) get(key []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	return getHelper(l, key, prefix, dao, bucket, cb)
}

func (l *leaf) upsert(
	key, value []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	return upsertHelper(l, key, value, prefix, dao, bucket, cb)
}

func (l *leaf) delete(key []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	return deleteHelper(l, key, prefix, dao, bucket, cb)
}

// child returns the node's child, and length of matching path
func (l *leaf) child(key []byte, dao db.KVStore, bucket string, cb db.CachedBatch) (patricia, int, error) {
	if l.Ext == len(l.Path) {
		return nil, 0, nil
	}
	match := 0
	for l.Path[l.Ext+match] == key[match] {
		match++
		if l.Ext+match == len(l.Path) {
			if l.Ext == EXTLEAF {
				child, err := getPatricia(l.Value, dao, bucket, cb)
				if err != nil {
					return nil, 0, errors.Wrapf(err, "failed to get child of key = %x", key)
				}
				return child, match, nil
			}
			return nil, match, nil
		}
	}
	return nil, match, ErrPathDiverge
}

// ascend updates the key as a result of child node update
func (l *leaf) ascend(key []byte, _ byte) error {
	// leaf node will be replaced by newly created node, no need to update hash
	if l.Ext != EXTLEAF {
		return errors.Wrap(ErrInvalidPatricia, "leaf should not exist on path ascending to root")
	}
	l.Value = nil
	if key != nil {
		l.Value = make([]byte, len(key))
		copy(l.Value, key)
	}
	return nil
}

// insert <k, v> at current patricia node
func (l *leaf) insert(k, v []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	divK := k[prefix:]
	// get the matching length on diverging path
	match := 0
	for l.Path[l.Ext+match] == divK[match] {
		match++
	}
	// insert() gets called b/c path does not totally match so the below should not happen, but check anyway
	if match == len(l.Path) {
		return nil, errors.Wrapf(ErrInvalidPatricia, "leaf already has total matching path = %x", l.Path)
	}
	// add leaf for new <k, v>
	l1 := leaf{prefix + match + 1, make([]byte, len(k)), v}
	copy(l1.Path, k)
	hashl, err := putPatriciaNew(&l1, bucket, cb)
	logger.Debug().Hex("newL", hashl[:8]).Int("prefix", l1.Ext).Hex("path", k[l1.Ext:]).Msg("splitL")
	if err != nil {
		return nil, err
	}
	// add 1 branch to link new leaf and current ext
	b := branch{}
	b.Path[divK[match]] = hashl
	if l.Ext == EXTLEAF {
		// split the current ext
		divE := l.Path[match:]
		switch len(divE) {
		case 1:
			b.Path[divE[0]] = l.Value
			logger.Debug().Hex("currL", l.Value[:8]).Hex("path", divE[0:1]).Msg("splitE")
		default:
			// add 1 ext to link to current ext
			e := leaf{EXTLEAF, divE[1:], l.Value}
			hashe, err := putPatriciaNew(&e, bucket, cb)
			if err != nil {
				return nil, err
			}
			logger.Debug().Hex("currE", hashe[:8]).Hex("k", divE[1:]).Hex("v", l.Value).Msg("splitE")
			// link new leaf and current ext (which becomes e)
			b.Path[divE[0]] = hashe
		}
	} else {
		l2 := leaf{l.Ext + match + 1, l.Path, l.Value}
		hashl, err := putPatriciaNew(&l2, bucket, cb)
		if err != nil {
			return nil, err
		}
		logger.Debug().Hex("currL", hashl[:8]).Int("prefix", l2.Ext).Hex("path", l2.Path[l2.Ext:]).Msg("splitL")
		b.Path[l2.Path[l2.Ext-1]] = hashl
	}
	hashb, err := putPatriciaNew(&b, bucket, cb)
	if err != nil {
		return nil, err
	}
	logger.Debug().Hex("newB", hashb[:8]).Msg("split")
	// if there's matching part, add 1 ext leading to top of split
	if match > 0 {
		e := leaf{EXTLEAF, l.Path[l.Ext : l.Ext+match], hashb}
		hashe := e.hash()
		logger.Debug().Hex("topE", hashe[:8]).Hex("path", e.Path).Msg("split")
		return putPatriciaNew(&e, bucket, cb)
	}
	return hashb[:], nil
}

// increase returns the number of nodes (B, E, L) being added as a result of insert()
func (l *leaf) increase(key []byte) (int, int, int) {
	// get the matching length
	match := 0
	for l.Path[l.Ext+match] == key[match] {
		match++
	}
	B, E, L := 1, 0, 0
	if l.Ext == EXTLEAF {
		L = 1
		switch len(l.Path[l.Ext+match:]) {
		case 1:
		default:
			E++
		}
		if match > 0 {
			E++
		}
	} else {
		L = 2
		if match > 0 {
			E++
		}
	}
	return B, E, L
}

// collapse checks whether the node can collapse with its child, returns the hash of the new node
func (l *leaf) collapse(
	child patricia, key []byte, prefix int, dao db.KVStore, bucket string, cb db.CachedBatch) ([]byte, error) {
	if key == nil {
		// the child node is removed, so can this node
		return nil, nil
	}
	if _, ok := child.(*leaf); ok {
		// delete from DB
		delPatricia(child, bucket, cb)
		if err := child.merge(l.Path); err != nil {
			return nil, err
		}
		logger.Debug().Msg("collapse 2 ext into 1")
		// put node (with updated hash) into DB
		return putPatriciaNew(child, bucket, cb)
	}
	// put node (with updated hash) into DB
	return putPatriciaNew(l, bucket, cb)
}

func (l *leaf) merge(k []byte) error {
	if l.Ext == EXTLEAF {
		l.Path = append(k, l.Path...)
		return nil
	}
	l.Ext -= len(k)
	return nil
}

// set assigns v to the node
func (l *leaf) set(v []byte) error {
	if l.Ext == EXTLEAF {
		return errors.Wrap(ErrInvalidPatricia, "ext should not be updated")
	}
	l.Value = nil
	l.Value = make([]byte, len(v))
	copy(l.Value, v)
	return nil
}

// blob return the <k, v> stored in the node
func (l *leaf) blob(key []byte) ([]byte, error) {
	if l.Ext == EXTLEAF {
		// ext node stores the hash to next patricia node
		return nil, errors.Wrap(ErrInvalidPatricia, "ext does not store value")
	}
	if !bytes.Equal(l.Path, key) {
		return nil, errors.Wrapf(ErrNotExist, "key = %x", key)
	}
	return l.Value, nil
}

// hash return the hash of this node
func (l *leaf) hash() hash.Hash32B {
	stream := append([]byte{byte(l.Ext)}, l.Path...)
	stream = append(stream, l.Value...)
	return blake2b.Sum256(stream)
}

func (l *leaf) toProto() *iproto.NodePb {
	return &iproto.NodePb{
		Node: &iproto.NodePb_Leaf{
			Leaf: &iproto.LeafPb{
				Ext:   uint32(l.Ext),
				Path:  l.Path,
				Value: l.Value,
			},
		},
	}
}

// serialize to bytes
func (l *leaf) serialize() ([]byte, error) {
	return proto.Marshal(l.toProto())
}

func (l *leaf) fromProto(pbLeaf *iproto.LeafPb) {
	l.Ext = int(pbLeaf.Ext)
	l.Path = pbLeaf.Path
	l.Value = pbLeaf.Value
}

// deserialize to leaf
func (l *leaf) deserialize(stream []byte) error {
	pbNode := iproto.NodePb{}
	if err := proto.Unmarshal(stream, &pbNode); err != nil {
		return err
	}
	if pbLeaf := pbNode.GetLeaf(); pbLeaf != nil {
		l.fromProto(pbLeaf)
		return nil
	}
	return errors.Wrap(ErrInvalidPatricia, "invalid node type")
}
