package trie

import (
	"bytes"
	"container/list"
	"encoding/gob"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
)

const RADIX = 256

var (
	// ErrInvalidPatricia: invalid operation
	ErrInvalidPatricia = errors.New("invalid patricia operation")

	// ErrPathDiverge: the path diverges
	ErrPathDiverge = errors.New("path diverges")
)

type (
	patricia interface {
		descend([]byte) ([]byte, int, error)
		ascend([]byte, byte) error
		insert([]byte, []byte, *list.List) error
		blob() ([]byte, error)
		hash() common.Hash32B // hash of this node
		serialize() ([]byte, error)
		deserialize([]byte) error
	}
	// key of next patricia node
	ptrcKey []byte
	// branch is the full node having 256 hashes for next level patricia node + hash of leaf node
	branch struct {
		Path  [RADIX]ptrcKey
		Value []byte
	}
	// extension is squashed path + hash of next patricia node
	ext struct {
		Path  ptrcKey
		Hashn []byte
	}
	// leaf is squashed path + actual value
	leaf struct {
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
	if len(node) > 0 {
		return node, 1, nil
	}
	return nil, 0, errors.Wrapf(ErrInvalidPatricia, "branch does not have path = %d", key[0])
}

// ascend updates the patricia node along the path to root
func (b *branch) ascend(key []byte, index byte) error {
	if b.Path[index] == nil {
		b.Path[index] = make([]byte, common.HashSize)
	}
	copy(b.Path[index], key)
	return nil
}

// insert <key, value> at current patricia node
func (b *branch) insert(key, value []byte, stack *list.List) error {
	node := b.Path[key[0]]
	if len(node) > 0 {
		errors.Wrapf(ErrInvalidPatricia, "branch already covers path = %d", key[0])
	}
	// create a new leaf
	l := leaf{key[1:], value}
	stack.PushBack(&l)
	return nil
}

// blob return the value stored in the node
func (b *branch) blob() ([]byte, error) {
	// extension node stores the hash to next patricia node
	return nil, errors.Wrap(ErrInvalidPatricia, "branch does not store value")
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
	// first byte denotes the type of patricia: 0-branch, 1-extension, 2-leaf
	return append([]byte{0}, stream.Bytes()...), nil
}

// deserialize to branch
func (b *branch) deserialize(stream []byte) error {
	// reset variable
	*b = branch{}
	dec := gob.NewDecoder(bytes.NewBuffer(stream[1:]))
	if err := dec.Decode(b); err != nil {
		return err
	}
	return nil
}

//======================================
// functions for extension
//======================================
// descend returns the key to retrieve next patricia, and length of matching path in bytes
func (e *ext) descend(key []byte) ([]byte, int, error) {
	match := 0
	for e.Path[match] == key[match] {
		match++
		if match == len(e.Path) {
			return e.Hashn, match, nil
		}
	}
	return nil, match, ErrPathDiverge
}

// ascend updates the patricia node along the path to root
func (e *ext) ascend(key []byte, index byte) error {
	if e.Hashn == nil {
		e.Hashn = make([]byte, common.HashSize)
	}
	copy(e.Hashn, key)
	return nil
}

// insert <key, value> at current patricia node
func (e *ext) insert(key, value []byte, stack *list.List) error {
	return nil
}

// blob return the value stored in the node
func (e *ext) blob() ([]byte, error) {
	// extension node stores the hash to next patricia node
	return nil, errors.Wrap(ErrInvalidPatricia, "extension does not store value")
}

// hash return the hash of this node
func (e *ext) hash() common.Hash32B {
	stream := append(e.Path, e.Hashn[:]...)
	return blake2b.Sum256(stream)
}

// serialize to bytes
func (e *ext) serialize() ([]byte, error) {
	stream := bytes.Buffer{}
	enc := gob.NewEncoder(&stream)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	// first byte denotes the type of patricia: 0-branch, 1-extension, 2-leaf
	return append([]byte{1}, stream.Bytes()...), nil
}

// deserialize to extension
func (e *ext) deserialize(stream []byte) error {
	// reset variable
	*e = ext{}
	dec := gob.NewDecoder(bytes.NewBuffer(stream[1:]))
	if err := dec.Decode(e); err != nil {
		return err
	}
	return nil
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

// ascend updates the patricia node along the path to root
func (l *leaf) ascend(key []byte, index byte) error {
	return errors.Wrapf(ErrInvalidPatricia, "leaf cannot be on path to root")
}

// insert <key, value> at current patricia node
func (l *leaf) insert(key, value []byte, stack *list.List) error {
	// get the matching length
	match := 0
	for l.Path[match] == key[match] {
		match++
	}
	// insert() gets called b/c path does not totally match so the below should not happen, but check anyway
	if match == len(l.Path) {
		return errors.Wrapf(ErrInvalidPatricia, "try to split a node with matching path = %x", l.Path)
	}
	// add 2 leaf, l1 is current node, l2 for new <key, value>
	l1 := leaf{l.Path[match+1:], l.Value}
	hashl1 := l1.hash()
	l2 := leaf{key[match+1:], value}
	hashl2 := l2.hash()
	// add 1 branch to link 2 new leaf
	b := branch{}
	b.Path[l.Path[match]] = hashl1[:]
	b.Path[key[match]] = hashl2[:]
	// if there's matching part, add 1 ext leading to new branch
	if match > 0 {
		hashb := b.hash()
		e := ext{key[:match], hashb[:]}
		stack.PushBack(&e)
	}
	stack.PushBack(&b)
	stack.PushBack(&l1)
	stack.PushBack(&l2)
	return nil
}

// blob return the value stored in the node
func (l *leaf) blob() ([]byte, error) {
	return l.Value, nil
}

// hash return the hash of this node
func (l *leaf) hash() common.Hash32B {
	stream := append(l.Path, l.Value...)
	return blake2b.Sum256(stream)
}

// serialize to bytes
func (l *leaf) serialize() ([]byte, error) {
	stream := bytes.Buffer{}
	enc := gob.NewEncoder(&stream)
	if err := enc.Encode(l); err != nil {
		return nil, err
	}
	// first byte denotes the type of patricia: 0-branch, 1-extension, 2-leaf
	return append([]byte{2}, stream.Bytes()...), nil
}

// deserialize to extension
func (l *leaf) deserialize(stream []byte) error {
	// reset variable
	*l = leaf{}
	dec := gob.NewDecoder(bytes.NewBuffer(stream[1:]))
	if err := dec.Decode(l); err != nil {
		return err
	}
	return nil
}
