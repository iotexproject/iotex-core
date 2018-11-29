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

	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

const radix = 256

type branchNode struct {
	children map[byte]hash.Hash32B
	ser      []byte
}

func newEmptyBranchNode() *branchNode {
	return &branchNode{children: map[byte]hash.Hash32B{}}
}

func newBranchNodeFromProtoPb(pb *iproto.BranchPb) *branchNode {
	b := newEmptyBranchNode()
	for _, n := range pb.Branches {
		b.children[byte(n.Index)] = byteutil.BytesTo32B(n.Path)
	}
	return b
}

func (b *branchNode) Children(ctx context.Context) ([]Node, error) {
	tc, ok := getSameKeyLenTrieContext(ctx)
	if !ok {
		return nil, errors.New("failed to get operation context")
	}
	children := []Node{}
	for i := range b.children {
		if c, err := b.child(tc, i); err != nil {
			return nil, err
		} else if c != nil {
			children = append(children, c)
		}
	}

	return children, nil
}

func (b *branchNode) Type() NodeType {
	return BRANCH
}

func (b *branchNode) Value() []byte {
	return nil
}

func (b *branchNode) delete(tc SameKeyLenTrieContext, key keyType, offset uint8) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("delete from a branch")
	offsetKey := key[offset]
	child, err := b.child(tc, offsetKey)
	if err != nil {
		return nil, err
	}
	newChild, err := child.delete(tc, key, offset+1)
	if err != nil {
		return nil, err
	}
	if newChild != nil {
		return b.updateChild(tc, offsetKey, newChild)
	}
	switch len(b.children) {
	case 1:
		panic("branch shouldn't have 0 child after deleting")
	case 2:
		if err := tc.DeleteNodeFromDB(b); err != nil {
			return nil, err
		}
		var orphan Node
		var orphanKey byte
		for i, h := range b.children {
			if i != offsetKey {
				orphanKey = i
				if orphan, err = tc.LoadNodeFromDB(h); err != nil {
					return nil, err
				}
				break
			}
		}
		if orphan == nil {
			panic("unexpected branch status")
		}
		switch node := orphan.(type) {
		case *extensionNode:
			return node.updatePath(
				tc,
				append([]byte{orphanKey}, node.path...),
			)
		case *leafNode:
			return node, nil
		default:
			return tc.newExtensionNodeAndPutIntoDB([]byte{orphanKey}, node)
		}
	default:
		return b.updateChild(tc, offsetKey, newChild)
	}
}

func (b *branchNode) upsert(tc SameKeyLenTrieContext, key keyType, offset uint8, value []byte) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("upsert into a branch")
	var newChild Node
	offsetKey := key[offset]
	child, err := b.child(tc, offsetKey)
	switch errors.Cause(err) {
	case nil:
		newChild, err = child.upsert(tc, key, offset+1, value)
	case ErrNotExist:
		newChild, err = tc.newLeafNodeAndPutIntoDB(key, value)
	default:
		return nil, err
	}

	return b.updateChild(tc, offsetKey, newChild)
}

func (b *branchNode) search(tc SameKeyLenTrieContext, key keyType, offset uint8) Node {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("search in a branch")
	child, err := b.child(tc, key[offset])
	if errors.Cause(err) == ErrNotExist {
		return nil
	}
	return child.search(tc, key, offset+1)
}

func (b *branchNode) serialize() []byte {
	if b.ser != nil {
		return b.ser
	}
	nodes := []*iproto.BranchNodePb{}
	for index := 0; index < radix; index++ {
		if h, ok := b.children[byte(index)]; ok {
			nodes = append(nodes, &iproto.BranchNodePb{Index: uint32(index), Path: h[:]})
		}
	}
	pb := &iproto.NodePb{
		Node: &iproto.NodePb_Branch{
			Branch: &iproto.BranchPb{Branches: nodes},
		},
	}
	ser, err := proto.Marshal(pb)
	if err != nil {
		logger.Panic().
			Err(err).
			Int("numOfChildren", len(b.children)).
			Interface("children", b.children).
			Msg("failed to marshal a branch node")
	}
	b.ser = ser

	return b.ser
}

func (b *branchNode) child(tc SameKeyLenTrieContext, key byte) (Node, error) {
	h, ok := b.children[key]
	if !ok {
		return nil, ErrNotExist
	}
	child, err := tc.LoadNodeFromDB(h)
	if err != nil {
		return nil, errors.Errorf("failed to fetch node for key %x", h[:])
	}
	return child, nil
}

func (b *branchNode) updateChild(
	tc SameKeyLenTrieContext,
	key byte,
	child Node,
) (*branchNode, error) {
	if err := tc.DeleteNodeFromDB(b); err != nil {
		return nil, err
	}
	b.ser = nil
	if child == nil {
		delete(b.children, key)
	} else {
		b.children[key] = nodeHash(child)
	}
	if err := tc.PutNodeIntoDB(b); err != nil {
		return nil, err
	}
	return b, nil
}
