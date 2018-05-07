package trie

import (
	"bytes"
	"encoding/gob"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/common"
)

const RADIX uint8 = 16

var (
	// ErrInvalidPatricia is the error for invalid operation
	ErrInvalidPatricia = errors.New("invalid patricia operation")
)

type (
	patricia interface {
		descend([]byte, bool) ([]byte, int, error)
		blob() ([]byte, error)
		hash() common.Hash32B // hash of this node
		serialize() ([]byte, error)
		deserialize([]byte) error
	}
	// key of next patricia node
	ptrcKey []byte
	// branch is the full node having 16 hashes for next level plus node value
	branch struct {
		Path  [RADIX]ptrcKey
		Value []byte
	}
	// extension is squashed path + hash of next patricia
	ext struct {
		Path  ptrcKey
		Value common.Hash32B
	}
	// leaf is squashed path + value
	leaf struct {
		Path  ptrcKey
		Value []byte
	}
)

// descend returns the key to retrieve next patricia, and length of matching path in nibbles (nibble = 4-bit)
func (b *branch) descend(key []byte, low bool) ([]byte, int, error) {
	var n uint8
	if low {
		n = common.ByteToNibbleLow(key[0])
	} else {
		n = common.ByteToNibbleHigh(key[0])
	}
	if len(b.Path[n]) > 0 {
		return b.Path[n], 1, nil
	}
	return nil, 0, errors.Wrapf(ErrInvalidPatricia, "branch does not have path = %d", n)
}

// blob return the value stored in the node
func (b *branch) blob() ([]byte, error) {
	return b.Value, nil
}

// hash return the hash of this node
func (b *branch) hash() common.Hash32B {
	stream := []byte{}
	for i := uint8(0); i < RADIX; i++ {
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
	// first byte denotes the type of next patricia: 0-branch, 1-extension, 2-leaf
	bytes := []byte{0}
	return append(bytes, stream.Bytes()...), nil
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

// descend returns the key to retrieve next patricia, and length of matching path in nibbles (nibble = 4-bit)
func (e *ext) descend(key []byte, low bool) ([]byte, int, error) {
	return nil, 0, nil
}

// blob return the value stored in the node
func (e *ext) blob() ([]byte, error) {
	// extension node stores the final hash to actual leaf
	return nil, errors.Wrap(ErrInvalidPatricia, "extension does not store value")
}

// hash return the hash of this node
func (e *ext) hash() common.Hash32B {
	stream := append(e.Path, e.Value[:]...)
	return blake2b.Sum256(stream)
}

// serialize to bytes
func (e *ext) serialize() ([]byte, error) {
	stream := bytes.Buffer{}
	enc := gob.NewEncoder(&stream)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	// first byte denotes the type of next patricia: 0-branch, 1-extension, 2-leaf
	bytes := []byte{1}
	return append(bytes, stream.Bytes()...), nil
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

// descend returns the key to retrieve next patricia, and length of matching path in nibbles (nibble = 4-bit)
func (l *leaf) descend(key []byte, low bool) ([]byte, int, error) {
	// descend should not be called on leaf node
	return nil, 0, errors.Wrap(ErrInvalidPatricia, "cannot descend further from leaf")
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
	// first byte denotes the type of next patricia: 0-branch, 1-extension, 2-leaf
	bytes := []byte{2}
	return append(bytes, stream.Bytes()...), nil
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
