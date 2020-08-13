// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math"
	"math/big"
	"time"
)

type votingPowerCalculator struct {
	DurationWeight    float64
	AutoStakingWeight float64
	SelfStakingWeight float64
}

func (vpc *votingPowerCalculator) calculate(v *VoteBucket, selfStake bool) *big.Int {
	remainingTime := v.StakedDuration.Seconds()
	weight := float64(1)
	var m float64
	if v.AutoStake {
		m = vpc.AutoStakingWeight
	}
	if remainingTime > 0 {
		weight += math.Log(math.Ceil(remainingTime/86400)*(1+m)) / math.Log(vpc.DurationWeight) / 100
	}
	if selfStake && v.AutoStake && v.StakedDuration >= time.Duration(91)*24*time.Hour {
		// self-stake extra bonus requires enable auto-stake for at least 3 months
		weight *= vpc.SelfStakingWeight
	}

	amount := new(big.Float).SetInt(v.StakedAmount)
	weightedAmount, _ := amount.Mul(amount, big.NewFloat(weight)).Int(nil)
	return weightedAmount
}
