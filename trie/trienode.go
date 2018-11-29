// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
)

type keyType []byte

// Node defines the interface of a trie node
// Note: all the key-value pairs should be of the same length of keys
type Node interface {
	Children(ctx context.Context) ([]Node, error)

	search(SameKeyLenTrieContext, keyType, uint8) Node
	delete(SameKeyLenTrieContext, keyType, uint8) (Node, error)
	upsert(SameKeyLenTrieContext, keyType, uint8, []byte) (Node, error)

	serialize() ([]byte, error)
}

func nodeHash(tn Node) (hash.Hash32B, error) {
	if tn == nil {
		return hash.ZeroHash32B, errors.New("no hash for nil node")
	}
	if bn, ok := tn.(*branchNode); ok && bn != nil && len(bn.children) == 0 {
		// Hack for unit test
		return EmptyRoot, nil
	}
	s, err := tn.serialize()
	if err != nil {
		return hash.ZeroHash32B, err
	}

	return blake2b.Sum256(s), nil
}

// key1 should not be longer than key2
func commonPrefixLength(key1, key2 []byte) uint8 {
	match := uint8(0)
	len1 := uint8(len(key1))
	for match < len1 && key1[match] == key2[match] {
		match++
	}

	return match
}

func deleteNodeFromDB(tc SameKeyLenTrieContext, tn Node) error {
	h, err := nodeHash(tn)
	if err != nil {
		return err
	}
	tc.CB.Delete(tc.Bucket, h[:], "failed to delete key = %x", h)
	return nil
}

func putNodeIntoDB(tc SameKeyLenTrieContext, tn Node) error {
	h, err := nodeHash(tn)
	if err != nil {
		return err
	}
	if h == hash.ZeroHash32B {
		return errors.New("zero hash is invalid")
	}
	if h == EmptyRoot {
		// Hack for unit test
		return nil
	}
	s, err := tn.serialize()
	if err != nil {
		return err
	}
	tc.CB.Put(tc.Bucket, h[:], s, "failed to put key = %x", h)
	return nil
}

func loadNodeFromDB(tc SameKeyLenTrieContext, key hash.Hash32B) (Node, error) {
	if key == hash.ZeroHash32B {
		return nil, errors.New("cannot fetch node for zero hash")
	}
	// Hack for unit test
	if key == EmptyRoot {
		return newEmptyBranchNode(), nil
	}
	s, err := tc.CB.Get(tc.Bucket, key[:])
	if err != nil {
		if s, err = tc.DB.Get(tc.Bucket, key[:]); err != nil {
			return nil, errors.Wrapf(err, "failed to get key %x", key[:8])
		}
	}
	pb := iproto.NodePb{}
	if err := proto.Unmarshal(s, &pb); err != nil {
		return nil, err
	}
	if pbBranch := pb.GetBranch(); pbBranch != nil {
		return newBranchNodeFromProtoPb(pbBranch), nil
	}
	if pbLeaf := pb.GetLeaf(); pbLeaf != nil {
		return newLeafNodeFromProtoPb(pbLeaf), nil
	}
	if pbExtend := pb.GetExtend(); pbExtend != nil {
		return newExtensionNodeFromProtoPb(pbExtend), nil
	}
	return nil, errors.Wrap(ErrInvalidPatricia, "invalid node type")
}
