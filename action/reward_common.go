// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// this struct is meant to return constant value for all rewarding actions, so we
// can use value receiver below
type reward_common struct{}

func (reward_common) EthTo() (*common.Address, error) {
	return &_rewardingProtocolEthAddr, nil
}

func (reward_common) Value() *big.Int { return nil }
