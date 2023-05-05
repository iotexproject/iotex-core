// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
)

type (
	// LiquidStakingStateReader defines the interface of liquid staking reader
	LiquidStakingStateReader interface {
		CandidateVotes(name string) *big.Int
	}

	emptyLiquidStakingStateReader struct{}
)

var _ LiquidStakingStateReader = (*emptyLiquidStakingStateReader)(nil)

func (f *emptyLiquidStakingStateReader) CandidateVotes(name string) *big.Int {
	return big.NewInt(0)
}
