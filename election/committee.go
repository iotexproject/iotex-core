// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package election

import (
	"math"
	"math/big"
	"time"

	"github.com/iotexproject/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const resultBucket = "height<->result"

// WeightFunc defines a function to calculate the weight of a vote
type WeightFunc func(*Vote) (*big.Int, error)

// CalcBeaconChainHeight calculates the corresponding beacon chain height for an epoch
type CalcBeaconChainHeight func(uint64) (uint64, error)

// Committee defines an interface of an election committee
type Committee interface {
	ResultByEpoch(epoch uint64) (*Result, error)
}

type committee struct {
	db         db.KVStore
	calcWeight WeightFunc
	carrier    VoteCarrier
	calcHeight CalcBeaconChainHeight
}

// NewCommittee creates a committee
func NewCommittee(cfg config.Election) (Committee, error) {
	if !common.IsHexAddress(cfg.StakingContractAddress) {
		return nil, errors.New("Invalid staking contract address")
	}
	calcWeight := func(v *Vote) (*big.Int, error) {
		// TODO: calculate weights
		now := time.Now()
		if now.Before(v.startTime) {
			return nil, errors.New("invalid vote start time")
		}
		// TODO: validate that duration is a positive value
		remainingTime := v.startTime.Add(v.duration).Sub(now).Seconds()
		weight := float64(1)
		if remainingTime > 0 {
			weight += math.Log(math.Ceil(remainingTime/86400)) / math.Log(1.2)
		}
		amount := new(big.Float).SetInt(v.amount)
		weightedAmount, _ := amount.Mul(amount, big.NewFloat(weight)).Int(nil)

		return weightedAmount, nil
	}
	calcHeight := func(epoch uint64) (uint64, error) {
		// TODO: calculate based on config
		return epoch, nil
	}
	return &committee{
		db:         db.NewOnDiskDB(cfg.DB),
		calcHeight: calcHeight,
		calcWeight: calcWeight,
		carrier: NewEthereumVoteCarrier(
			cfg.EthRawURL,
			common.HexToAddress(cfg.StakingContractAddress),
		),
	}, nil
}

// ResultByEpoch returns the result on a specific ethereum height
func (ec *committee) ResultByEpoch(epoch uint64) (*Result, error) {
	height, err := ec.calcHeight(epoch)
	if err != nil {
		return nil, err
	}
	heightHash := byteutil.Uint64ToBytes(height)
	data, err := ec.db.Get(resultBucket, heightHash)
	switch errors.Cause(err) {
	case db.ErrNotExist: // not exist in db
		break
	case nil: // in db
		result := &Result{}
		return result, result.Deserialize(data)
	default: // unexpected error
		return nil, err
	}
	var previousIndex *big.Int
	// TODO: move into config
	paginationSize := uint8(100)
	result := &Result{}
	for {
		var votes []*Vote
		if previousIndex, votes, err = ec.carrier.Votes(height, previousIndex, paginationSize); err != nil {
			return nil, err
		}
		for _, vote := range votes {
			if vote.candidate == "" {
				continue
			}
			weight, err := ec.calcWeight(vote)
			if err != nil {
				return nil, err
			}
			result.AddPoints(vote.candidate, weight)
		}
		if len(votes) < int(paginationSize) {
			break
		}
	}
	if data, err = result.Serialize(); err != nil {
		return nil, err
	}
	if err = ec.db.Put(resultBucket, heightHash, data); err != nil {
		return nil, err
	}

	return result, nil
}
