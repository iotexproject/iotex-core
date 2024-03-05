// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestTransientStorage(t *testing.T) {
	r := require.New(t)

	ts := newTransientStorage()
	r.NotNil(ts)
	var (
		addr0 = common.Address{}
		addr1 = common.HexToAddress("1234567890")
		k1    = common.Hash{}
		k2    = common.HexToHash("34567890ab")
		v1    = common.HexToHash("567890abcd")
		v2    = common.HexToHash("7890abcdef")
	)
	// new ts is empty
	h := ts.Get(addr0, k1)
	r.Zero(h)
	h = ts.Get(addr1, k2)
	r.Zero(h)
	// test Set and Get
	for _, v := range []struct {
		addr     common.Address
		key, val common.Hash
	}{
		{addr0, k1, v1},
		{addr0, k2, v2},
		{addr1, k1, v2},
		{addr1, k2, v1},
	} {
		ts.Set(v.addr, v.key, v.val)
		h = ts.Get(v.addr, v.key)
		r.Equal(v.val, h)
	}
	// test Copy
	ts1 := ts.Copy()
	for _, v := range []struct {
		addr     common.Address
		key, val common.Hash
	}{
		{addr0, k1, v1},
		{addr0, k2, v2},
		{addr1, k1, v2},
		{addr1, k2, v1},
	} {
		h = ts1.Get(v.addr, v.key)
		r.Equal(v.val, h)
	}
}
