// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math"

	"github.com/iotexproject/iotex-core/action/protocol/staking"
)

const (
	maxBlockNumber uint64 = math.MaxUint64
)

type (
	// ContractStakingBucketInfo is the bucket information
	ContractStakingBucketInfo = staking.ContractStakingBucketInfo

	// ContractStakingBucketType is the bucket type
	ContractStakingBucketType = staking.ContractStakingBucketType

	// ContractStakingBucket is the bucket information including bucket type and bucket info
	ContractStakingBucket = staking.VoteBucket
)
