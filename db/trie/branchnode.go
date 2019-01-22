// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-core/db/trie/triepb"
)

const radix = 256

type branchNode struct {
	children map[byte]Node
	cache    nodeCache
}

func newEmptyBranchNode() *branchNode {
	return &branchNode{children: map[byte]Node{}}
}

func newBranchNode(children map[byte]Node) *branchNode {
	bnode := newEmptyBranchNode()
	for i, n := range children {
		bnode.children[i] = n
		bnode.cache.dirty = true
	}
	return bnode
}

func (b *branchNode) Type() NodeType {
	return BRANCH
}

func (b *branchNode) Children() []Node {
	trieMtc.WithLabelValues("branchNode", "children").Inc()
	children := []Node{}
	for i := 0; i < radix; i++ {
		if c, ok := b.children[byte(i)]; ok {
			children = append(children, c)
		}
	}

	return children
}

func (b *branchNode) delete(tr Trie, key keyType, offset uint8) (Node, error) {
	trieMtc.WithLabelValues("branchNode", "delete").Inc()
	offsetKey := key[offset]
	child, ok := b.children[offsetKey]
	if !ok {
		return nil, ErrNotExist
	}
	newChild, err := child.delete(tr, key, offset+1)
	if err != nil {
		return nil, err
	}
	if newChild != nil {
		return b.updateChild(offsetKey, newChild), nil
	}
	switch len(b.children) {
	case 1:
		panic("branch shouldn't have 0 child after deleting")
	case 2:
		var orphan Node
		var orphanKey byte
		for i, child := range b.children {
			if i != offsetKey {
				orphanKey = i
				orphan = child
				break
			}
		}
		if orphan == nil {
			panic("unexpected branch status")
		}
		if hn, ok := orphan.(*hashNode); ok {
			h, err := hn.hash(tr, false)
			if err != nil {
				return nil, err
			}
			if orphan, err = tr.LoadNodeFromDB(h); err != nil {
				return nil, err
			}
		}
		switch node := orphan.(type) {
		case *leafNode:
			return node, nil
		case *extensionNode:
			return node.updatePath(append([]byte{orphanKey}, node.path...)), nil
		default:
			return newExtensionNode([]byte{orphanKey}, node), nil
		}
	default:
		return b.updateChild(offsetKey, newChild), nil
	}
}

func (b *branchNode) upsert(tr Trie, key keyType, offset uint8, value []byte) (Node, error) {
	trieMtc.WithLabelValues("branchNode", "upsert").Inc()
	var newChild Node
	offsetKey := key[offset]
	child, ok := b.children[offsetKey]
	if ok {
		var err error
		if newChild, err = child.upsert(tr, key, offset+1, value); err != nil {
			return nil, err
		}
	} else {
		newChild = newLeafNode(key, value)
	}

	return b.updateChild(offsetKey, newChild), nil
}

func (b *branchNode) search(tr Trie, key keyType, offset uint8) (Node, error) {
	trieMtc.WithLabelValues("branchNode", "search").Inc()
	child, ok := b.children[key[offset]]
	if !ok {
		return nil, ErrNotExist
	}
	return child.search(tr, key, offset+1)
}

func (b *branchNode) isDirty() bool {
	return b.cache.dirty
}

func (b *branchNode) hash(tr Trie, flush bool) ([]byte, error) {
	if b.cache.hn != nil {
		if b.isDirty() && flush {
			for index := 0; index < radix; index++ {
				if child, ok := b.children[byte(index)]; ok {
					if _, err := child.hash(tr, flush); err != nil {
						return nil, err
					}
				}
			}
		}
	} else {
		s, err := b.serialize(tr, flush)
		if err != nil {
			return nil, err
		}
		b.cache.hn = newHashNodeFromSer(tr, s)
	}
	if b.isDirty() && flush {
		if err := tr.putNodeIntoDB(b.cache.hn); err != nil {
			return nil, err
		}
	}

	return b.cache.hn.getHash(), nil
}

type hashData struct {
	index byte
	hash  []byte
	err   error
}

func (b *branchNode) serialize(tr Trie, flush bool) ([]byte, error) {
	trieMtc.WithLabelValues("branchNode", "serialize").Inc()
	hashChan := make(chan *hashData)
	wg := sync.WaitGroup{}
	wg.Add(len(b.children))
	go func() {
		wg.Wait()
		close(hashChan)
	}()
	for i, child := range b.children {
		go func(index byte, node Node) {
			defer wg.Done()
			childHash, err := node.hash(tr, flush)
			hashChan <- &hashData{err: err, index: index, hash: childHash}
		}(i, child)
	}
	hashes := map[byte][]byte{}
	for data := range hashChan {
		if data.err != nil {
			return nil, data.err
		}
		hashes[data.index] = data.hash
	}
	branches := []*triepb.BranchPb{}
	for index := 0; index < radix; index++ {
		if h, ok := hashes[byte(index)]; ok {
			branches = append(
				branches,
				&triepb.BranchPb{
					Index:     uint32(index),
					ChildHash: h,
				},
			)
		}
	}
	return proto.Marshal(&triepb.NodePb{
		Node: &triepb.NodePb_Branch{
			Branch: &triepb.BranchNodePb{
				Branches: branches,
			},
		},
	})
}

func (b *branchNode) updateChild(key byte, child Node) *branchNode {
	children := map[byte]Node{}
	for i, c := range b.children {
		children[i] = c
	}
	if child == nil {
		delete(children, key)
	} else {
		children[key] = child
	}
	return newBranchNode(children)
}
