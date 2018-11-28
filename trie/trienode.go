// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"
	"context"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"golang.org/x/crypto/blake2b"
)

type keyType [20]byte

type trieNode interface {
	Children(context.Context) []trieNode
	Search(context.Context, keyType, int) trieNode

	Delete(context.Context, keyType, int) (trieNode, error)
	Upsert(context.Context, keyType, int, []byte) (trieNode, error)

	Serialize() []byte
	Deserialize([]byte) error
	Value() []byte
}

func hashTrieNode(tn trieNode) hash.Hash32B {
	return blake2b.Sum256(tn.Serialize())
}

type leafNode struct {
	key   keyType
	value []byte
}

func (l *leafNode) Children(context.Context) []trieNode {
	return []trieNode{}
}

func (l *leafNode) Delete(ctx context.Context, key keyType, offset int) (trieNode, error) {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil, ErrNotExist
	}
	// delete current node from db
	return nil, nil
}

func (l *leafNode) Upsert(ctx context.Context, key keyType, offset int, value []byte) (trieNode, error) {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		// l.split(key)
		return nil, nil
	}
	l.value = value
	// update current node in db

	return l, nil
}

func (l *leafNode) Search(ctx context.Context, key keyType, offset int) trieNode {
	if !bytes.Equal(l.key[offset:], key[offset:]) {
		return nil
	}

	return l
}

func (l *leafNode) Serialize() []byte {
	return nil
}

func (l *leafNode) Deserialize(s []byte) error {
	return nil
}

func (l *leafNode) Value() []byte {
	return l.value
}

type branchNode struct {
	children [RADIX]hash.Hash32B
}

func (b *branchNode) Children(ctx context.Context) []trieNode {
	children := []trieNode{}
	for idx := 0; idx < RADIX; idx++ {
		if c := b.child(ctx, idx); c != nil {
			children = append(children, c)
		}
		idx++
	}

	return children
}

func (b *branchNode) child(ctx context.Context, key int) trieNode {
	if b.children[key] == hash.ZeroHash32B {
		return nil
	}
	// load trieNode with ctx
	return nil
}

func (b *branchNode) Delete(ctx context.Context, key keyType, offset int) (trieNode, error) {
	ok := int(key[offset])
	child := b.child(ctx, ok)
	if child == nil {
		return nil, ErrNotExist
	}
	newChild, err := child.Delete(ctx, key, offset+1)
	if err != nil {
		return nil, err
	}
	if newChild != nil {
		b.children[ok] = hashTrieNode(newChild)
		return b, nil
	}
	b.children[ok] = hash.ZeroHash32B
	cnt := 0
	for _, chash := range b.children {
		if chash != hash.ZeroHash32B {
			cnt++
			if cnt >= 2 {
				return b, nil
			}
		}
	}
	if cnt == 0 {
		panic("branch shouldn't have 0 child after deleting")
	}
	switch newChild.(type) {
	case *extendNode:
		extendChild := newChild.(*extendNode)
		// delete current node from db
		extendChild.path = append(key[offset:offset+1], extendChild.path...)
		return extendChild, nil
	case *leafNode:
		// delete current node from db
		return newChild, nil
	default:
		return &extendNode{
			path:      key[offset : offset+1],
			childHash: hashTrieNode(newChild),
		}, nil
	}
}

func (b *branchNode) Upsert(ctx context.Context, key keyType, offset int, value []byte) (trieNode, error) {
	var newChild trieNode
	var err error
	ok := int(key[offset])
	if child := b.child(ctx, ok); child == nil {
		newChild = &leafNode{
			key:   key,
			value: value,
		}
		// write into db
	} else if newChild, err = child.Upsert(ctx, key, offset+1, value); err != nil {
		return nil, err
	}
	b.children[ok] = hashTrieNode(newChild)
	// update current node in db

	return b, nil
}

func (b *branchNode) Search(ctx context.Context, key keyType, offset int) trieNode {
	child := b.child(ctx, int(key[offset]))
	if child == nil {
		return nil
	}
	return child.Search(ctx, key, offset+1)
}

func (b *branchNode) Serialize() []byte {
	return nil
}

func (b *branchNode) Deserialize(s []byte) error {
	return nil
}

func (b *branchNode) Value() []byte {
	return nil
}

type extendNode struct {
	path      []byte
	childHash hash.Hash32B
}

func (e *extendNode) Children(context.Context) []trieNode {
	return []trieNode{e.child()}
}

func (e *extendNode) child() trieNode {
	// load child from db
	return nil
}

func (e *extendNode) match(key []byte) int {
	match := 0
	for match < len(e.path) && e.path[match] == key[match] {
		match++
	}

	return match
}

func (e *extendNode) Delete(ctx context.Context, key keyType, offset int) (trieNode, error) {
	matched := e.match(key[offset:])
	if matched != len(e.path) {
		return nil, ErrNotExist
	}
	child := e.child()
	newChild, err := child.Delete(ctx, key, offset+matched)
	if err != nil {
		return nil, err
	}
	if newChild == nil {
		// delete current node from db
		return nil, nil
	}
	switch newChild.(type) {
	case *extendNode:
		extendChild := newChild.(*extendNode)
		extendChild.path = append(e.path, extendChild.path...)
		// delete current node from db
		// update extendChild in db
		return extendChild, nil
	case *leafNode:
		// delete current node from db
		return newChild, nil
	default:
		e.childHash = hashTrieNode(newChild)
		// update current node in db
		return e, nil
	}
}

func (e *extendNode) Upsert(ctx context.Context, key keyType, offset int, value []byte) (trieNode, error) {
	matched := e.match(key[offset:])
	if matched != len(e.path) {
		b := &branchNode{}
		eNode := &extendNode{
			path:      e.path[matched+1:],
			childHash: e.childHash,
		}
		// put into db
		lNode := &leafNode{key: key, value: value}
		// put into db
		b.children[eNode.path[matched]] = hashTrieNode(eNode)
		b.children[key[offset+matched]] = hashTrieNode(lNode)
		// put into db
		if matched == 0 {
			return b, nil
		}
		e.path = e.path[:matched]
		e.childHash = hashTrieNode(b)
		// delete old and update new in db
		return e, nil
	}
	child := e.child()
	newChild, err := child.Upsert(ctx, key, offset+matched, value)
	if err != nil {
		return nil, err
	}
	e.childHash = hashTrieNode(newChild)
	// update current node in db

	return e, nil
}

func (e *extendNode) Search(ctx context.Context, key keyType, offset int) trieNode {
	matched := e.match(key[offset:])
	if matched != len(e.path) {
		return nil
	}

	return e.child().Search(ctx, key, offset+matched)
}

func (e *extendNode) Serialize() []byte {
	return nil
}

func (e *extendNode) Deserialize(s []byte) error {
	return nil
}

func (e *extendNode) Value() []byte {
	return nil
}
