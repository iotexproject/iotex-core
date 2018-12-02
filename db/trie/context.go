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

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	iproto "github.com/iotexproject/iotex-core/proto"
)

type sameKeyLenTrieContextKey struct{}

// SameKeyLenTrieContext provides the trie operation with auxiliary information.
type SameKeyLenTrieContext struct {
	// Bucket defines the bucket to store in db
	Bucket string
	// DB refers to the db to store the key-value pairs
	DB db.KVStore
	// CB is a collection of db operations
	CB db.CachedBatch
	// KeyLength defines the length of the key to be used in the trie
	KeyLength int
	// InitRootHash stores the root hash
	InitRootHash hash.Hash32B
	// RootKey is the db key storing the root hash
	RootKey string
}

// WithSameKeyLenTrieContext add SameKeyLenTrieContext into context.
func WithSameKeyLenTrieContext(ctx context.Context, tc SameKeyLenTrieContext) context.Context {
	return context.WithValue(ctx, sameKeyLenTrieContextKey{}, tc)
}

func getSameKeyLenTrieContext(ctx context.Context) (SameKeyLenTrieContext, bool) {
	tc, ok := ctx.Value(sameKeyLenTrieContextKey{}).(SameKeyLenTrieContext)

	return tc, ok
}

// Commit commits changes to db
func (tc *SameKeyLenTrieContext) Commit() error {
	return tc.DB.Commit(tc.CB)
}

func (tc *SameKeyLenTrieContext) newLeafNodeAndPutIntoDB(
	key keyType,
	value []byte,
) (*leafNode, error) {
	l := newLeafNode(key, value)
	if err := tc.PutNodeIntoDB(l); err != nil {
		return nil, err
	}
	return l, nil
}

func (tc *SameKeyLenTrieContext) newBranchNodeAndPutIntoDB(
	children map[byte]Node,
) (*branchNode, error) {
	bnode := newEmptyBranchNode()
	for i, n := range children {
		if n == nil {
			continue
		}
		bnode.children[i] = nodeHash(n)
	}
	if err := tc.PutNodeIntoDB(bnode); err != nil {
		return nil, err
	}
	return bnode, nil
}

func (tc *SameKeyLenTrieContext) newExtensionNodeAndPutIntoDB(
	path []byte,
	child Node,
) (*extensionNode, error) {
	e := &extensionNode{path: path, childHash: nodeHash(child)}
	if err := tc.PutNodeIntoDB(e); err != nil {
		return nil, err
	}
	return e, nil
}

// DeleteNodeFromDB deletes a node from db
func (tc *SameKeyLenTrieContext) DeleteNodeFromDB(tn Node) error {
	h := nodeHash(tn)
	tc.CB.Delete(tc.Bucket, h[:], "failed to delete key = %x", h)
	return nil
}

// PutNodeIntoDB stores a node into db
func (tc *SameKeyLenTrieContext) PutNodeIntoDB(tn Node) error {
	h := nodeHash(tn)
	if h == EmptyBranchNodeHash {
		return nil
	}
	s := tn.serialize()
	tc.CB.Put(tc.Bucket, h[:], s, "failed to put key = %x", h)
	return nil
}

// LoadNodeFromDB loads node from db with hash
func (tc *SameKeyLenTrieContext) LoadNodeFromDB(key hash.Hash32B) (Node, error) {
	if key == EmptyBranchNodeHash {
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
	return nil, errors.New("invalid node type")
}
