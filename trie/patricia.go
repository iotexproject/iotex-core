// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
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
		descend([]byte) ([]byte, int, error)
		ascend([]byte, byte) error
		insert([]byte, []byte, int, *list.List) error
		increase([]byte) (int, int, int)
		collapse([]byte, byte) ([]byte, []byte, int, error)
		set([]byte) error
		blob() ([]byte, []byte, error)
		hash() hash.Hash32B // hash of this node
		serialize() ([]byte, error)
		deserialize([]byte) error
	}
	// key of next patricia node
	ptrcKey []byte
	// branch is the full node storing 256 hashes of next level node
	branch struct {
		Split bool
		Path  [RADIX]ptrcKey
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
		b.Path[index] = make([]byte, len(key))
		copy(b.Path[index], key)
	}
	return nil
}

// insert <k, v> at current patricia node
func (b *branch) insert(k, v []byte, prefix int, stack *list.List) error {
	divK := k[prefix:]
	node := b.Path[divK[0]]
	if node != nil {
		return errors.Wrapf(ErrInvalidPatricia, "branch already has path = %d", k[0])
	}
	// create a new leaf
	l := leaf{prefix + 1, make([]byte, len(k)), v}
	copy(l.Path, k)
	stack.PushBack(&l)
	h := l.hash()
	logger.Debug().Hex("newL", h[:8]).Int("prefix", l.Ext).Hex("path", k[l.Ext:]).Msg("splitB")
	b.Split = true
	return nil
}

// increase returns the number of nodes (B, E, L) being added as a result of insert()
func (b *branch) increase(key []byte) (int, int, int) {
	return 0, 0, 1
}

// collapse updates the node, returns the path and key if the node can be collapsed
func (b *branch) collapse(key []byte, index byte) ([]byte, []byte, int, error) {
	if key == nil {
		b.Path[index] = nil
		// count number of remaining path
		nb := 0
		path := byte(0)
		for i := 0; i < RADIX; i++ {
			if len(b.Path[i]) > 0 {
				nb++
				key = b.Path[i]
				path = byte(i)
			}
		}
		logger.Debug().Uint8("branch", index).Msg("trim")
		return []byte{path}, key, nb, nil
	}
	return nil, nil, 2, b.ascend(key, index)
}

// set assigns v to the node
func (b *branch) set(v []byte) error {
	return errors.Wrapf(ErrInvalidPatricia, "set() should not be called on branch")
}

// blob return the <k, v> stored in the node
func (b *branch) blob() ([]byte, []byte, error) {
	// branch node stores the hash to next patricia node
	return nil, nil, errors.Wrap(ErrInvalidPatricia, "branch does not store value")
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

//======================================
// functions for leaf
//======================================
// descend returns the key to retrieve next patricia, and length of matching path in bytes
func (l *leaf) descend(key []byte) ([]byte, int, error) {
	match := 0
	for l.Path[l.Ext+match] == key[match] {
		match++
		if l.Ext+match == len(l.Path) {
			return l.Value, match, nil
		}
	}
	return nil, match, ErrPathDiverge
}

// ascend updates the key as a result of child node update
func (l *leaf) ascend(key []byte, index byte) error {
	// leaf node will be replaced by newly created node, no need to update hash
	if l.Ext > EXTLEAF {
		return errors.Wrap(ErrInvalidPatricia, "leaf should not exist on path ascending to root")
	}
	l.Value = nil
	l.Value = make([]byte, len(key))
	copy(l.Value, key)
	return nil
}

// insert <k, v> at current patricia node
func (l *leaf) insert(k, v []byte, prefix int, stack *list.List) error {
	divK := k[prefix:]
	// get the matching length on diverging path
	match := 0
	for l.Path[l.Ext+match] == divK[match] {
		match++
	}
	// insert() gets called b/c path does not totally match so the below should not happen, but check anyway
	if match == len(l.Path) {
		return errors.Wrapf(ErrInvalidPatricia, "leaf already has total matching path = %x", l.Path)
	}
	// add leaf for new <k, v>
	l1 := leaf{prefix + match + 1, make([]byte, len(k)), v}
	copy(l1.Path, k)
	hashl := l1.hash()
	logger.Debug().Hex("newL", hashl[:8]).Int("prefix", l1.Ext).Hex("path", k[l1.Ext:]).Msg("splitL")
	// add 1 branch to link new leaf and current ext
	b := branch{}
	b.Path[divK[match]] = hashl[:]
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
			hashe := e.hash()
			logger.Debug().Hex("currE", hashe[:8]).Hex("k", divE[1:]).Hex("v", l.Value).Msg("splitE")
			// link new leaf and current ext (which becomes e)
			b.Path[divE[0]] = hashe[:]
			stack.PushBack(&e)
		}
	} else {
		l2 := leaf{l.Ext + match + 1, l.Path, l.Value}
		hashl := l2.hash()
		logger.Debug().Hex("currL", hashl[:8]).Int("prefix", l2.Ext).Hex("path", l2.Path[l2.Ext:]).Msg("splitL")
		b.Path[l2.Path[l2.Ext-1]] = hashl[:]
		stack.PushBack(&l2)
	}
	stack.PushBack(&l1)
	stack.PushFront(&b)
	hashb := b.hash()
	logger.Debug().Hex("newB", hashb[:8]).Msg("split")
	// if there's matching part, add 1 ext leading to top of split
	if match > 0 {
		e := leaf{EXTLEAF, l.Path[l.Ext : l.Ext+match], hashb[:]}
		stack.PushFront(&e)
		hashe := e.hash()
		logger.Debug().Hex("topE", hashe[:8]).Hex("path", e.Path).Msg("split")
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
	if l.Ext == EXTLEAF {
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

// collapse updates the node, returns the path and key if the node can be collapsed
func (l *leaf) collapse(key []byte, index byte) ([]byte, []byte, int, error) {
	if key == nil {
		// the child node is removed, so can this ext
		return nil, nil, 0, nil
	}
	return l.Path, l.Value, 1, l.ascend(key, index)
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
func (l *leaf) blob() ([]byte, []byte, error) {
	if l.Ext == EXTLEAF {
		// ext node stores the hash to next patricia node
		return nil, nil, errors.Wrap(ErrInvalidPatricia, "ext does not store value")
	}
	return l.Path, l.Value, nil
}

// hash return the hash of this node
func (l *leaf) hash() hash.Hash32B {
	stream := append([]byte{byte(l.Ext)}, l.Path...)
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
	// first byte denotes the type of patricia: 1-branch, 0-leaf
	return append([]byte{EXTLEAF}, stream.Bytes()...), nil
}

// deserialize to leaf
func (l *leaf) deserialize(stream []byte) error {
	// reset variable
	*l = leaf{}
	dec := gob.NewDecoder(bytes.NewBuffer(stream[1:]))
	return dec.Decode(l)
}
