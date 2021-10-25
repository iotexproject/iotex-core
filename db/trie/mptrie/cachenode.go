// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type cacheNode struct {
	dirty bool
	serializable
	mpt     *merklePatriciaTrie
	hashVal []byte
	ser     []byte
}

func (cn *cacheNode) Print(indent string) {
	h, _ := cn.Hash()
	fmt.Printf("%s%x\n", indent, h)
}

func (cn *cacheNode) Hash() ([]byte, error) {
	return cn.hash(false)
}

func (cn *cacheNode) hash(flush bool) ([]byte, error) {
	if cn.hashVal != nil {
		return cn.hashVal, nil
	}
	pb, err := cn.proto(flush)
	if err != nil {
		return nil, err
	}
	ser, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}

	cn.ser = ser
	cn.hashVal = cn.mpt.hashFunc(ser)

	return cn.hashVal, nil
}

func (cn *cacheNode) delete() error {
	h, err := cn.hash(false)
	if err != nil {
		return err
	}
	if !cn.dirty {
		if err := cn.mpt.deleteNode(h); err != nil {
			return err
		}
	}
	cn.hashVal = nil
	cn.ser = nil

	return nil
}

func (cn *cacheNode) store() (node, error) {
	h, err := cn.hash(true)
	if err != nil {
		return nil, err
	}
	if cn.dirty {
		if err := cn.mpt.putNode(h, cn.ser); err != nil {
			return nil, err
		}
		cn.dirty = false
	}
	return newHashNode(cn.mpt, h), nil
}
