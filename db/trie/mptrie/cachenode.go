// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"google.golang.org/protobuf/proto"
)

type cacheNode struct {
	dirty bool
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
	pb, err := cn.proto()
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
	if !cn.dirty {
		h, err := cn.hash()
		if err != nil {
			return err
		}
		if err := cn.mpt.deleteNode(h); err != nil {
			return err
		}
	}
	cn.hashVal = nil
	cn.ser = nil

	return nil
}

func (cn *cacheNode) store() error {
	if cn.dirty {
		h, err := cn.hash()
		if err != nil {
			return err
		}
		if err := cn.mpt.putNode(h, cn.ser); err != nil {
			return err
		}
		cn.dirty = false
	}
	return nil
}
