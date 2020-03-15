// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package merklepatriciatree

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// ErrNoData is an error when a hash node has no corresponding data
var ErrNoData = errors.New("no data in hash node")

type (
	keyType []byte
	node    interface {
		Search(keyType, uint8) (node, error)
		Delete(keyType, uint8) (node, error)
		Upsert(keyType, uint8, []byte) (node, error)
		Hash() ([]byte, error)
	}

	serializable interface {
		node
		hash() ([]byte, error)
		proto() (proto.Message, error)
		delete() error
		store() (node, error)
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

	cacheNode struct {
		serializable
		//serializable
		mpt *merklePatriciaTree
		ha  []byte
		ser []byte
	}
)

func (cn *cacheNode) Hash() ([]byte, error) {
	return cn.hash()
}

func (cn *cacheNode) hash() ([]byte, error) {
	if cn.ha != nil {
		return cn.ha, nil
	}
	if err := cn.calculateCache(); err != nil {
		return nil, err
	}

	return cn.ha, nil
}

func (cn *cacheNode) delete() error {
	h, err := cn.hash()
	if err != nil {
		return err
	}
	if err := cn.mpt.deleteNode(h); err != nil {
		return err
	}
	cn.ha = nil
	cn.ser = nil

	return nil
}

func (cn *cacheNode) store() (node, error) {
	ser, err := cn.serialize()
	if err != nil {
		return nil, err
	}
	h, err := cn.hash()
	if err != nil {
		return nil, err
	}
	if err := cn.mpt.putNode(h, ser); err != nil {
		return nil, err
	}
	return newHashNode(cn.mpt, h), nil
}

func (cn *cacheNode) calculateCache() error {
	pb, err := cn.proto()
	if err != nil {
		return err
	}
	ser, err := proto.Marshal(pb)
	if err != nil {
		return err
	}
	cn.ser = ser
	cn.ha = cn.mpt.hashFunc(ser)
	return nil
}

func (cn *cacheNode) serialize() ([]byte, error) {
	if cn.ser != nil {
		return cn.ser, nil
	}
	if err := cn.calculateCache(); err != nil {
		return nil, err
	}

	return cn.ser, nil
}

// key1 should not be longer than key2
func commonPrefixLength(key1, key2 []byte) uint8 {
	match := uint8(0)
	len1 := uint8(len(key1))
	for match < len1 && key1[match] == key2[match] {
		match++
	}

	return match
}
