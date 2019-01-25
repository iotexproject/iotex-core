// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package election

import (
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/iotexproject/go-ethereum/accounts/abi/bind"
	"github.com/iotexproject/go-ethereum/common"
	"github.com/iotexproject/go-ethereum/ethclient"
)

// Vote defines the structure of a vote
type Vote struct {
	startTime time.Time
	duration  time.Duration
	amount    *big.Int
	voter     common.Address
	candidate string
	// TODO: add nodecay feature
}

// VoteCarrier defines an interfact to fetch votes
type VoteCarrier interface {
	Votes(uint64, *big.Int, uint8) (*big.Int, []*Vote, error)
}

type etherVoteCarrier struct {
	url          string
	contractAddr common.Address
}

// NewEthereumVoteCarrier defines a carrier to fetch votes from ethereum contract
func NewEthereumVoteCarrier(url string, contractAddr common.Address) VoteCarrier {
	return &etherVoteCarrier{
		contractAddr: contractAddr,
		url:          url,
	}
}

func (evc *etherVoteCarrier) Votes(
	height uint64,
	previousIndex *big.Int,
	count uint8,
) (*big.Int, []*Vote, error) {
	if previousIndex == nil || previousIndex.Cmp(big.NewInt(0)) < 0 {
		previousIndex = big.NewInt(0)
	}
	client, err := ethclient.Dial(evc.url)
	if err != nil {
		return nil, nil, err
	}
	caller, err := NewStakingContractCaller(evc.contractAddr, client)
	if err != nil {
		return nil, nil, err
	}
	buckets, err := caller.GetActiveBuckets(
		&bind.CallOpts{BlockNumber: new(big.Int).SetUint64(height)},
		previousIndex,
		big.NewInt(int64(count)),
	)
	if err != nil {
		return nil, nil, err
	}
	votes := []*Vote{}
	num := len(buckets.Indexes)
	if num == 0 {
		return previousIndex, votes, nil
	}
	candidateAddrs := strings.Split(buckets.StakedFor, "|")
	for i, index := range buckets.Indexes {
		if big.NewInt(0).Cmp(index) == 0 {
			break
		}
		if len(candidateAddrs) <= i+1 {
			return nil, nil, errors.New("invalid return value")
		}
		votes = append(votes, &Vote{
			startTime: time.Unix(buckets.StakeStartTime[i].Int64(), 0),
			duration:  time.Duration(buckets.StakeDuration[i].Uint64()*24) * time.Hour,
			amount:    buckets.StakedAmount[i],
			voter:     buckets.Owners[i],
			candidate: candidateAddrs[i+1],
		})
		if index.Cmp(previousIndex) > 0 {
			previousIndex = index
		}
	}

	return previousIndex, votes, nil
}
