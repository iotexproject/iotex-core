// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"github.com/ethereum/go-ethereum/common"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessList(t *testing.T) {
	r := require.New(t)

	al := newAccessList()
	r.NotNil(al)

	addrTests := []struct {
		addr       common.Address
		addrChange bool
	}{
		{c1, true},
		{c2, true},
		{c1, false},
	}

	for _, v := range addrTests {
		r.Equal(v.addrChange, al.AddAddress(v.addr))
	}

	slotTests := []struct {
		addr       common.Address
		addrChange bool
		slot       common.Hash
		slotChange bool
	}{
		{c1, false, k1, true},
		{c1, false, k2, true},
		{c1, false, k1, false},
		{c3, true, k1, true},
		{c3, false, k2, true},
		{c3, false, k2, false},
	}

	for _, v := range slotTests {
		a, s := al.AddSlot(v.addr, v.slot)
		r.Equal(v.addrChange, a)
		r.Equal(v.slotChange, s)
	}

	containTests := []struct {
		addr      common.Address
		addrExist bool
		slots     []common.Hash
		nxSlot    common.Hash
	}{
		{c1, true, []common.Hash{k1, k2}, k3},
		{c3, true, []common.Hash{k1, k2}, k4},
		{c2, true, nil, k1},
		{C4, false, nil, k2},
	}

	for _, v := range containTests {
		for _, e := range v.slots {
			a, s := al.Contains(v.addr, e)
			r.True(a)
			r.True(s)
		}
		a, s := al.Contains(v.addr, v.nxSlot)
		r.Equal(v.addrExist, a)
		r.False(s)
	}
}
