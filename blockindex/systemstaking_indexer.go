// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
)

type (
	// SystemStakingIndexer is the interface of system staking indexer
	SystemStakingIndexer interface {
		blockdao.BlockIndexer

		GetCandidateVotes(candidate string) (*big.Int, error)
		GetBucket(bucketIndex uint64) (*staking.VoteBucket, error)
	}

	systemStakingIndexer struct{}
)

// NewSystemStakingIndexer creates a new system staking indexer
func NewSystemStakingIndexer() SystemStakingIndexer {
	return &systemStakingIndexer{}
}

func (s *systemStakingIndexer) Start(ctx context.Context) error {
	return nil
}

func (s *systemStakingIndexer) Stop(ctx context.Context) error {
	return nil
}

func (s *systemStakingIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	return nil
}

func (s *systemStakingIndexer) DeleteTipBlock(context.Context, *block.Block) error {
	return nil
}

func (s *systemStakingIndexer) Height() (uint64, error) {
	return 0, nil
}

func (s *systemStakingIndexer) GetCandidateVotes(candidate string) (*big.Int, error) {
	return nil, nil
}

func (s *systemStakingIndexer) GetBucket(bucketIndex uint64) (*staking.VoteBucket, error) {
	return nil, nil
}
