// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
)

// Bucket defines the bucket struct for contract staking
type Bucket struct {
	Index                     uint64
	Candidate                 address.Address
	Owner                     address.Address
	StakedAmount              *big.Int
	StakedDurationBlockNumber uint64
	CreateBlockHeight         uint64
	StakeStartBlockHeight     uint64
	UnstakeStartBlockHeight   uint64
	AutoStake                 bool
	ContractAddress           string // contract address for the bucket
}

func assembleBucket(token uint64, bi *bucketInfo, bt *BucketType, contractAddr string) (*Bucket, error) {
	vb := Bucket{
		Index:                     token,
		StakedAmount:              bt.Amount,
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
	return &vb, nil
}
