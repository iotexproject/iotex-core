// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

// Bucket defines the bucket struct for contract staking
type Bucket = staking.VoteBucket

func assembleBucket(token uint64, bi *bucketInfo, bt *BucketType, contractAddr string, blocksToDurationFn blocksDurationFn) *Bucket {
	vb := Bucket{
		Index:                     token,
		StakedAmount:              bt.Amount,
		StakedDuration:            blocksToDurationFn(bi.CreatedAt, bi.CreatedAt+bt.Duration),
		StakedDurationBlockNumber: bt.Duration,
		CreateBlockHeight:         bi.CreatedAt,
		StakeStartBlockHeight:     bi.CreatedAt,
		UnstakeStartBlockHeight:   bi.UnstakedAt,
		AutoStake:                 bi.UnlockedAt == maxBlockNumber,
		Candidate:                 bi.Delegate,
		Owner:                     bi.Owner,
		ContractAddress:           contractAddr,
	}
	if bi.UnlockedAt != maxBlockNumber {
		vb.StakeStartBlockHeight = bi.UnlockedAt
	}
	return &vb
}
