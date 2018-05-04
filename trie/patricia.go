package trie

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/common"
)

const RADIX uint8 = 16

var (
	// ErrInvalidBlock is the error returned when the block is not valid
	ErrInvalidPatricia = errors.New("invalid patricia operation")
)

type (
	patricia interface {
		descend([]byte, int) (patricia, int, error)
		blob() ([]byte, error)
		hash() common.Hash32B // hash of this node
	}
	// branch is the full node having 16 hashes for next level plus node value
	branch struct {
		node  [RADIX]common.Hash32B
		value []byte
	}
	// extension is path + next hash (which stores the final value)
	ext struct {
		path  []byte
		value common.Hash32B
	}
	// leaf is path + value
	leaf struct {
		path  []byte
		value []byte
	}
)

// descend return the next entry walking the patricia
func (b *branch) descend(key []byte, length int) (patricia, int, error) {
	return &ext{nil, b.node[0]}, 0, nil
}

// blob return the value stored in the node
func (b *branch) blob() ([]byte, error) {
	return b.value, nil
}

// hash return the hash of this node
func (b *branch) hash() common.Hash32B {
	return common.Hash32B{}
}

// descend return the next entry walking the patricia
func (e *ext) descend(key []byte, length int) (patricia, int, error) {
	return e, 0, nil
}

// blob return the value stored in the node
func (e *ext) blob() ([]byte, error) {
	// extension node stores the final hash to actual leaf
	return nil, errors.Wrap(ErrInvalidPatricia, "extension does not store value")
}

// hash return the hash of this node
func (e *ext) hash() common.Hash32B {
	return common.Hash32B{}
}

// descend return the next entry walking the patricia
func (l *leaf) descend(key []byte, length int) (patricia, int, error) {
	// descend should not be called on leaf node
	return nil, 0, errors.Wrap(ErrInvalidPatricia, "cannot descend further from leaf")
}

// blob return the value stored in the node
func (l *leaf) blob() ([]byte, error) {
	return l.value, nil
}

// hash return the hash of this node
func (l *leaf) hash() common.Hash32B {
	return common.Hash32B{}
}
