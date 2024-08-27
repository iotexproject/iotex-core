// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

// AbstractAction is an abstract implementation of Action interface
type AbstractActionAccessList struct {
	version    uint32
	chainID    uint32
	nonce      uint64
	gasLimit   uint64
	gasPrice   *big.Int
	accessList types.AccessList
}

// TODO
// all the funcs
// Version(), ChainID(), Nonce(), GasLimit(), AccessList, toProto(), fromProto(), SanityCheck()

func (*AbstractActionAccessList) AccessList() {}
