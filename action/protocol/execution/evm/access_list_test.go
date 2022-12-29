// Copyright (c) 2021 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"

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
		{_c1, true},
		{_c2, true},
		{_c1, false},
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
		{_c1, false, _k1, true},
		{_c1, false, _k2, true},
		{_c1, false, _k1, false},
		{_c3, true, _k1, true},
		{_c3, false, _k2, true},
		{_c3, false, _k2, false},
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
		{_c1, true, []common.Hash{_k1, _k2}, _k3},
		{_c3, true, []common.Hash{_k1, _k2}, _k4},
		{_c2, true, nil, _k1},
		{_c4, false, nil, _k2},
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
