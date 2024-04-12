// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/hash"
)

type (
	// test vector for Contract
	set struct {
		k     hash.Hash256
		v     []byte
		cause error
	}
	cntrTest struct {
		contract Contract
		codes    []code
		states   []set
	}
	// test vector for StateDBAdapter
	bal struct {
		addr common.Address
		v    *big.Int
	}
	code struct {
		addr common.Address
		v    []byte
	}
	evmSet struct {
		addr common.Address
		k    common.Hash
		v    common.Hash
	}
	sui struct {
		amount       *big.Int
		beneficiary  common.Address
		addr         common.Address
		selfDestruct bool
		exist        bool
	}
	image struct {
		hash common.Hash
		v    []byte
	}
	access struct {
		addr  common.Address
		slots []common.Hash
		nx    []common.Hash
		exist bool
	}
	transient struct {
		addr common.Address
		k    common.Hash
		v    common.Hash
	}
	stateDBTest struct {
		balance                           []bal
		codes                             []code
		states                            []evmSet
		refund                            uint64
		selfDestruct                      []sui
		preimage                          []image
		accessList                        []access
		transient                         []transient
		logs                              []*types.Log
		logSize, txLogSize, transientSize int
		logAddr, txSender, txReceiver     string
	}
)

var (
	_bytecode = []byte("test contract creation")

	_addr1 = common.HexToAddress("02ae2a956d21e8d481c3a69e146633470cf625ec")
	_c1    = common.HexToAddress("01fc246633470cf62ae2a956d21e8d481c3a69e1")
	_c2    = common.HexToAddress("3470cf62ae2a956d38d481c3a69e121e01fc2466")
	_c3    = common.HexToAddress("956d21e8d481c3a6901fc246633470cf62ae2ae1")
	_c4    = common.HexToAddress("121e01fc24663470cf62ae2a956d38d481c3a69e")

	_k1b = hash.Hash256b([]byte("cat"))
	_v1b = hash.Hash256b([]byte("cat"))
	_k2b = hash.Hash256b([]byte("dog"))
	_v2b = hash.Hash256b([]byte("dog"))
	_k3b = hash.Hash256b([]byte("hen"))
	_v3b = hash.Hash256b([]byte("hen"))
	_k4b = hash.Hash256b([]byte("fox"))
	_v4b = hash.Hash256b([]byte("fox"))
	_k1  = common.BytesToHash(_k1b[:])
	_v1  = common.BytesToHash(_v1b[:])
	_k2  = common.BytesToHash(_k2b[:])
	_v2  = common.BytesToHash(_v2b[:])
	_k3  = common.BytesToHash(_k3b[:])
	_v3  = common.BytesToHash(_v3b[:])
	_k4  = common.BytesToHash(_k4b[:])
	_v4  = common.BytesToHash(_v4b[:])
)

func newTestLog(addr common.Address) *types.Log {
	return &types.Log{
		Address: addr,
		Topics:  []common.Hash{_k1},
	}
}
