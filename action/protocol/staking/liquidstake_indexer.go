// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
)

type (
	// LiquidStakingIndexer defines the interface of liquid staking reader
	LiquidStakingIndexer interface {
		CandidateVotes(ownerAddr string) *big.Int
	}

	// TODO (iip-13): remove this empty liquid staking indexer
	emptyLiquidStakingIndexer struct{}
)

var _ LiquidStakingIndexer = (*emptyLiquidStakingIndexer)(nil)

// NewEmptyLiquidStakingIndexer creates an empty liquid staking indexer
func NewEmptyLiquidStakingIndexer() LiquidStakingIndexer {
	return &emptyLiquidStakingIndexer{}
}

func (f *emptyLiquidStakingIndexer) CandidateVotes(ownerAddr string) *big.Int {
	return big.NewInt(0)
}
