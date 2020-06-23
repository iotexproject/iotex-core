// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"github.com/golang/protobuf/proto"
)

type cacheNode struct {
	serializable
	mpt     *merklePatriciaTrie
	hashVal []byte
	ser     []byte
}

func (cn *cacheNode) Hash() ([]byte, error) {
	return cn.hash()
}

func (cn *cacheNode) hash() ([]byte, error) {
	if cn.hashVal != nil {
		return cn.hashVal, nil
	}
	if err := cn.calculateCache(); err != nil {
		return nil, err
	}
	return cn.hashVal, nil
}

func (cn *cacheNode) delete() error {
	h, err := cn.hash()
	if err != nil {
		return err
	}
	if err := cn.mpt.deleteNode(h); err != nil {
		return err
	}
	cn.hashVal = nil
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
	cn.hashVal = cn.mpt.hashFunc(ser)
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
