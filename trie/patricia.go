// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"bytes"
	"container/list"
	"encoding/gob"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/logger"
)

// RADIX specifies the number of unique digits in patricia
const RADIX = 256

var (
	// ErrInvalidPatricia indicates invalid operation
	ErrInvalidPatricia = errors.New("invalid patricia operation")

	// ErrPathDiverge indicates the path diverges
	ErrPathDiverge = errors.New("path diverges")
)

type (
	patricia interface {
		descend([]byte) ([]byte, int, error)
		ascend([]byte, byte) error
		insert([]byte, []byte, *list.List) error
		increase([]byte) (int, int, int)
		collapse([]byte, []byte, byte, bool) ([]byte, []byte, bool)
		set([]byte, byte) error
		blob() ([]byte, []byte, error)
		hash() common.Hash32B // hash of this node
		serialize() ([]byte, error)
		deserialize([]byte) error
	}
	// key of next patricia node
	ptrcKey []byte
	// branch is the full node having 256 hashes for next level patricia node + hash of leaf node
	branch struct {
		Split bool
		Path  [RADIX]ptrcKey
		Value []byte
	}
	// leaf is squashed path + actual value (or hash of next patricia node for extension)
	leaf struct {
		Ext   byte // this is an extension node
		Path  ptrcKey
		Value []byte
	}
)

//======================================
// functions for branch
//======================================
// descend returns the key to retrieve next patricia, and length of matching path in bytes
func (b *branch) descend(key []byte) ([]byte, int, error) {
	node := b.Path[key[0]]
	if node != nil {
		return node, 1, nil
	}
	return nil, 0, errors.Wrapf(ErrInvalidPatricia, "branch does not have path = %d", key[0])
}

// ascend updates the key as a result of child node update
func (b *branch) ascend(key []byte, index byte) error {
	if b.Path[index] != nil || b.Split {
		b.Split = false
		b.Path[index] = nil
		b.Path[index] = make([]byte, common.HashSize)
		copy(b.Path[index], key)
	}
	return nil
}

// insert <k, v> at current patricia node
func (b *branch) insert(k, v []byte, stack *list.List) error {
	node := b.Path[k[0]]
	if node != nil {
		return errors.Wrapf(ErrInvalidPatricia, "branch already has path = %d", k[0])
	}
	// create a new leaf
	l := leaf{0, k[1:], v}
	stack.PushBack(&l)
	b.Split = true
	return nil
}

// increase returns the number of nodes (B, E, L) being added as a result of insert()
func (b *branch) increase(key []byte) (int, int, int) {
	return 0, 0, 1
}

// collapse updates the node, returns the <key, value> if the node can be collapsed
// value is the hash of only remaining leaf node, another DB access is needed to get the actual value
func (b *branch) collapse(k, v []byte, index byte, childClps bool) ([]byte, []byte, bool) {
	// if child cannot collapse, no need to check and return false
	if !childClps {
		return k, v, false
	}
	// value == nil means no entry exist on the incoming path, trim it
	if v == nil {
		b.Path[index] = nil
	}
	// count number of remaining path
	nb := 0
	var key, value []byte
	for i := 0; i < RADIX; i++ {
		if len(b.Path[i]) > 0 {
			nb++
			key = nil
			key = []byte{byte(i)}
			value = b.Path[i]
		}
	}
	// branch can be collapsed if less than 1 path remaining
	if nb <= 1 {
		if v == nil {
			// incoming path is trimmed, set k to nil and v to value
			// so it returns the remaining path <key, value> 4 lines below
			k = nil
			v = value
		}
		return append(key, k...), v, true
	}
	return k, v, false
}

// set assigns v to the node
func (b *branch) set(v []byte, index byte) error {
	if b.Path[index] != nil {
		return errors.Wrapf(ErrInvalidPatricia, "branch already has path = %d", index)
	}
	b.Path[index] = make([]byte, common.HashSize)
	copy(b.Path[index], v)
	return nil
}

// blob return the <k, v> stored in the node
func (b *branch) blob() ([]byte, []byte, error) {
	// branch node stores the hash to next patricia node
	return nil, nil, errors.Wrap(ErrInvalidPatricia, "branch does not store value")
}

// hash return the hash of this node
func (b *branch) hash() common.Hash32B {
	stream := []byte{}
	for i := 0; i < RADIX; i++ {
		stream = append(stream, b.Path[i]...)
	}
	stream = append(stream, b.Value...)
	return blake2b.Sum256(stream)
}

// serialize to bytes
func (b *branch) serialize() ([]byte, error) {
	var stream bytes.Buffer
	enc := gob.NewEncoder(&stream)
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	// first byte denotes the type of patricia: 2-branch, 1-extension, 0-leaf
	return append([]byte{2}, stream.Bytes()...), nil
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

//======================================
// functions for leaf
//======================================
// descend returns the key to retrieve next patricia, and length of matching path in bytes
func (l *leaf) descend(key []byte) ([]byte, int, error) {
	match := 0
	for l.Path[match] == key[match] {
		match++
		if match == len(l.Path) {
			return l.Value, match, nil
		}
	}
	return nil, match, ErrPathDiverge
}

// ascend updates the key as a result of child node update
func (l *leaf) ascend(key []byte, index byte) error {
	// leaf node will be replaced by newly created node, no need to update hash
	if l.Ext == 0 {
		return errors.Wrap(ErrInvalidPatricia, "leaf should not exist on path ascending to root")
	}
	l.Value = nil
	l.Value = make([]byte, common.HashSize)
	copy(l.Value, key)
	return nil
}

// insert <k, v> at current patricia node
func (l *leaf) insert(k, v []byte, stack *list.List) error {
	// get the matching length
	match := 0
	for l.Path[match] == k[match] {
		match++
	}
	// insert() gets called b/c path does not totally match so the below should not happen, but check anyway
	if match == len(l.Path) {
		return errors.Wrapf(ErrInvalidPatricia, "leaf already has total matching path = %x", l.Path)
	}
	if l.Ext == 1 {
		// split the current ext
		logger.Debug().Hex("new key", k[match:]).Msg("diverge")
		if err := l.split(match, k[match:], v, stack); err != nil {
			return err
		}
		n := stack.Front()
		ptr, _ := n.Value.(patricia)
		hash := ptr.hash()
		//======================================
		// the matching part becomes a new ext leading to top of split
		// new E <P[:match]> -> top of split
		//======================================
		if match > 0 {
			e := leaf{Ext: 1}
			e.Path = l.Path[:match]
			e.Value = hash[:]
			hashe := e.hash()
			logger.Debug().Hex("topE", hashe[:8]).Hex("path", l.Path[:match]).Msg("splitL")
			stack.PushFront(&e)
		}
		return nil
	}
	// add 2 leaf, l1 is current node, l2 for new <key, value>
	l1 := leaf{0, l.Path[match+1:], l.Value}
	hashl1 := l1.hash()
	l2 := leaf{0, k[match+1:], v}
	hashl2 := l2.hash()
	// add 1 branch to link 2 new leaf
	b := branch{}
	b.Path[l.Path[match]] = hashl1[:]
	b.Path[k[match]] = hashl2[:]
	stack.PushBack(&b)
	stack.PushBack(&l1)
	stack.PushBack(&l2)
	// if there's matching part, add 1 ext leading to new branch
	if match > 0 {
		hashb := b.hash()
		e := leaf{1, k[:match], hashb[:]}
		stack.PushFront(&e)
	}
	return nil
}

// increase returns the number of nodes (B, E, L) being added as a result of insert()
func (l *leaf) increase(key []byte) (int, int, int) {
	// get the matching length
	match := 0
	for l.Path[match] == key[match] {
		match++
	}
	B, E, L := 1, 0, 0
	if l.Ext == 1 {
		L = 1
		switch len(l.Path[match:]) {
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

// collapse updates the node, returns the <key, value> if the node can be collapsed
func (l *leaf) collapse(k, v []byte, index byte, childCollapse bool) ([]byte, []byte, bool) {
	// if child cannot collapse, no need to check and return false
	if !childCollapse {
		return k, v, false
	}
	return append(l.Path, k...), v, true
}

// set assigns v to the node
func (l *leaf) set(v []byte, index byte) error {
	if l.Ext == 1 {
		return errors.Wrap(ErrInvalidPatricia, "ext should not be updated")
	}
	l.Value = nil
	l.Value = make([]byte, len(v))
	copy(l.Value, v)
	return nil
}

// blob return the <k, v> stored in the node
func (l *leaf) blob() ([]byte, []byte, error) {
	if l.Ext == 1 {
		// ext node stores the hash to next patricia node
		return nil, nil, errors.Wrap(ErrInvalidPatricia, "ext does not store value")
	}
	return l.Path, l.Value, nil
}

// hash return the hash of this node
func (l *leaf) hash() common.Hash32B {
	stream := append([]byte{l.Ext}, l.Path...)
	stream = append(stream, l.Value...)
	return blake2b.Sum256(stream)
}

// serialize to bytes
func (l *leaf) serialize() ([]byte, error) {
	stream := bytes.Buffer{}
	enc := gob.NewEncoder(&stream)
	if err := enc.Encode(l); err != nil {
		return nil, err
	}
	// first byte denotes the type of patricia: 2-branch, 1-extension, 0-leaf
	return append([]byte{l.Ext}, stream.Bytes()...), nil
}

// deserialize to leaf
func (l *leaf) deserialize(stream []byte) error {
	// reset variable
	*l = leaf{}
	dec := gob.NewDecoder(bytes.NewBuffer(stream[1:]))
	return dec.Decode(l)
}

// split diverging path
//======================================
// len(k) == 1
// E -> B[P[0]] -> E.value>
//      B[k[0]] -> Leaf <k[1:], v> this is the <k, v> to be inserted
// len(k) > 1
// E -> B[P[0]] -> E <P[1:]], E.value>
//      B[k[0]] -> Leaf <k[1:], v> this is the <k, v> to be inserted
//======================================
func (l *leaf) split(match int, k, v []byte, stack *list.List) error {
	var node patricia
	divPath := l.Path[match:]
	logger.Debug().Hex("curr key", divPath).Msg("diverge")
	// add leaf for new <k, v>
	l1 := leaf{0, k[1:], v}
	hashl := l1.hash()
	logger.Debug().Hex("L", hashl[:8]).Hex("path", k[1:]).Msg("splitL")
	// add 1 branch to link new leaf and current ext (which may split as below)
	b := branch{}
	b.Path[k[0]] = hashl[:]
	switch len(divPath) {
	case 1:
		b.Path[divPath[0]] = l.Value
		logger.Warn().Hex("L", hashl[:8]).Hex("path", divPath[0:1]).Msg("splitL")
	default:
		// add 1 ext to split current ext
		e := leaf{1, divPath[1:], l.Value}
		hashe := e.hash()
		logger.Debug().Hex("E", hashe[:8]).Hex("k", divPath[1:]).Hex("v", l.Value).Msg("splitL")
		// link new leaf and current ext (which becomes e)
		b.Path[divPath[0]] = hashe[:]
		node = &e
	}
	hashb := b.hash()
	stack.PushBack(&b)
	logger.Debug().Hex("B", hashb[:8]).Hex("path", k[0:1]).Msg("splitL")
	if node != nil {
		stack.PushBack(node)
	}
	stack.PushBack(&l1)
	return nil
}
