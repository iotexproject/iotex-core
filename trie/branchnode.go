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

type branchNode struct {
	children map[byte]hash.Hash32B
	ser      []byte
}

func newBranchNodeAndSave(
	tc SameKeyLenTrieContext, children map[byte]Node,
) (*branchNode, error) {
	bnode := newEmptyBranchNode()
	for i, n := range children {
		if n == nil {
			continue
		}
		h, err := nodeHash(n)
		if err != nil {
			return nil, err
		}
		bnode.children[i] = h
	}
	if err := putNodeIntoDB(tc, bnode); err != nil {
		return nil, err
	}
	return bnode, nil
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

func (b *branchNode) delete(tc SameKeyLenTrieContext, key keyType, offset uint8) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("delete from a branch")
	ok := key[offset]
	child, err := b.child(tc, ok)
	if err != nil {
		return nil, err
	}
	newChild, err := child.delete(tc, key, offset+1)
	if err != nil {
		return nil, err
	}
	if newChild != nil {
		return b.updateChild(tc, ok, newChild)
	}
	switch len(b.children) {
	case 1:
		panic("branch shouldn't have 0 child after deleting")
	case 2:
		if err := deleteNodeFromDB(tc, b); err != nil {
			return nil, err
		}
		var orphan Node
		var orphanKey byte
		for i, h := range b.children {
			if i != ok {
				orphanKey = i
				if orphan, err = loadNodeFromDB(tc, h); err != nil {
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
			return newExtensionNodeAndSave(tc, []byte{orphanKey}, node)
		}
	default:
		return b.updateChild(tc, ok, newChild)
	}
}

func (b *branchNode) upsert(tc SameKeyLenTrieContext, key keyType, offset uint8, value []byte) (Node, error) {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("upsert into a branch")
	var newChild Node
	ok := key[offset]
	child, err := b.child(tc, ok)
	if err != nil {
		if errors.Cause(err) != ErrNotExist {
			return nil, err
		}
		newChild, err = newLeafNodeAndSave(tc, key, value)
	} else {
		newChild, err = child.upsert(tc, key, offset+1, value)
	}
	if err != nil {
		return nil, err
	}

	return b.updateChild(tc, ok, newChild)
}

func (b *branchNode) search(tc SameKeyLenTrieContext, key keyType, offset uint8) Node {
	logger.Debug().Hex("key", key[:]).Uint8("offset", offset).Msg("search in a branch")
	child, err := b.child(tc, key[offset])
	if errors.Cause(err) == ErrNotExist {
		return nil
	}
	return child.search(tc, key, offset+1)
}

func (b *branchNode) serialize() ([]byte, error) {
	if b.ser != nil {
		return b.ser, nil
	}
	nodes := []*iproto.BranchNodePb{}
	for index := 0; index < RADIX; index++ {
		if h, ok := b.children[byte(index)]; ok {
			nodes = append(nodes, &iproto.BranchNodePb{Index: uint32(index), Path: h[:]})
		}
	}
	pb := &iproto.NodePb{
		Node: &iproto.NodePb_Branch{
			Branch: &iproto.BranchPb{Branches: nodes},
		},
	}

	var err error
	b.ser, err = proto.Marshal(pb)

	return b.ser, err
}

func (b *branchNode) child(tc SameKeyLenTrieContext, key byte) (Node, error) {
	h, ok := b.children[key]
	if !ok {
		return nil, ErrNotExist
	}
	child, err := loadNodeFromDB(tc, h)
	if err != nil {
		return nil, errors.Errorf("failed to fetch node for key %x", h[:])
	}
	return child, nil
}

func (b *branchNode) updateChild(
	tc SameKeyLenTrieContext, key byte,
	child Node,
) (*branchNode, error) {
	if err := deleteNodeFromDB(tc, b); err != nil {
		return nil, err
	}
	b.ser = nil
	if child == nil {
		delete(b.children, key)
	} else {
		h, err := nodeHash(child)
		if err != nil {
			return nil, err
		}
		b.children[key] = h
	}
	if err := putNodeIntoDB(tc, b); err != nil {
		return nil, err
	}
	return b, nil
}
