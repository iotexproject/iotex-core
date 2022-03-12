// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// ErrNoData is an error when a hash node has no corresponding data
var ErrNoData = errors.New("no data in hash node")

type (
	keyType []byte

	node interface {
		Search(keyType, uint8) (node, error)
		Delete(keyType, uint8) (node, error)
		Upsert(keyType, uint8, []byte) (node, error)
		Hash() ([]byte, error)
		Flush() error
	}

	serializable interface {
		node
		hash() ([]byte, error)
		proto() (proto.Message, error)
		delete() error
		store() error
	}

	leaf interface {
		node
		// Key returns the key of a node, only leaf has key
		Key() []byte
		// Value returns the value of a node, only leaf has value
		Value() []byte
	}

	extension interface {
		node
		Child() node
	}

	branch interface {
		node
		Children() []node
		MarkAsRoot()
	}
)

// key1 should not be longer than key2
func commonPrefixLength(key1, key2 []byte) uint8 {
	match := uint8(0)
	len1 := uint8(len(key1))
	for match < len1 && key1[match] == key2[match] {
		match++
	}

	return match
}
