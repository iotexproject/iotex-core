// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package carrier

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/iotexproject/iotex-election/contract"
	"github.com/iotexproject/iotex-election/types"
)

// Carrier defines an interfact to fetch votes
type Carrier interface {
	// BlockTimestamp returns the timestamp of a block
	BlockTimestamp(uint64) (time.Time, error)
	// SubscribeNewBlock callbacks on new block created
	SubscribeNewBlock(func(uint64), chan bool) error
	// Candidates returns the candidates on height
	Candidates(uint64, *big.Int, uint8) (*big.Int, []*types.Candidate, error)
	// Votes returns the votes on height
	Votes(uint64, *big.Int, uint8) (*big.Int, []*types.Vote, error)
}

type ethereumCarrier struct {
	client                  *ethclient.Client
	stakingContractAddress  common.Address
	registerContractAddress common.Address
}

// NewEthereumVoteCarrier defines a carrier to fetch votes from ethereum contract
func NewEthereumVoteCarrier(
	url string,
	registerContractAddress common.Address,
	stakingContractAddress common.Address,
) (Carrier, error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil, err
	}
	return &ethereumCarrier{
		client:                  client,
		stakingContractAddress:  stakingContractAddress,
		registerContractAddress: registerContractAddress,
	}, nil
}

func (evc *ethereumCarrier) Close() {
	evc.client.Close()
}

func (evc *ethereumCarrier) BlockTimestamp(height uint64) (time.Time, error) {
	header, err := evc.client.HeaderByNumber(context.Background(), big.NewInt(0).SetUint64(height))
	if err != nil {
		return time.Now(), err
	}
	return time.Unix(header.Time.Int64(), 0), nil
}

func (evc *ethereumCarrier) SubscribeNewBlock(cb func(uint64), close chan bool) error {
	headers := make(chan *ethtypes.Header)
	sub, err := evc.client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case closed := <-close:
				close <- closed
				break
			case err := <-sub.Err():
				log.Fatal(err)
			case header := <-headers:
				fmt.Printf("New block %d %x\n", header.Number, header.Hash())
				cb(header.Number.Uint64())
			}
		}
	}()
	return nil
}

func (evc *ethereumCarrier) Candidates(
	height uint64,
	startIndex *big.Int,
	count uint8,
) (*big.Int, []*types.Candidate, error) {
	if startIndex == nil || startIndex.Cmp(big.NewInt(1)) < 0 {
		startIndex = big.NewInt(1)
	}
	caller, err := contract.NewRegisterCaller(evc.registerContractAddress, evc.client)
	if err != nil {
		return nil, nil, err
	}
	retval, err := caller.GetAllCandidates(
		&bind.CallOpts{BlockNumber: new(big.Int).SetUint64(height)},
		startIndex,
		big.NewInt(int64(count)),
	)
	if err != nil {
		return nil, nil, err
	}
	num := len(retval.Names)
	if len(retval.Addresses) != num {
		return nil, nil, errors.New("invalid addresses from GetAllCandidates")
	}
	operatorPubKeys, err := decodePubKeys(retval.IoOperatorPubKeys, num)
	if err != nil {
		return nil, nil, err
	}
	rewardPubKeys, err := decodePubKeys(retval.IoRewardPubKeys, num)
	if err != nil {
		return nil, nil, err
	}
	candidates := make([]*types.Candidate, num)
	for i := 0; i < num; i++ {
		candidates[i] = types.NewCandidate(
			retval.Names[i][:],
			retval.Addresses[i][:],
			operatorPubKeys[i],
			rewardPubKeys[i],
			1, // TODO: read weight from contract
		)
	}
	return new(big.Int).Add(startIndex, big.NewInt(int64(num))), candidates, nil
}

func (evc *ethereumCarrier) Votes(
	height uint64,
	previousIndex *big.Int,
	count uint8,
) (*big.Int, []*types.Vote, error) {
	if previousIndex == nil || previousIndex.Cmp(big.NewInt(0)) < 0 {
		previousIndex = big.NewInt(0)
	}
	caller, err := contract.NewStakingCaller(evc.stakingContractAddress, evc.client)
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
	votes := []*types.Vote{}
	num := len(buckets.Indexes)
	if num == 0 {
		return previousIndex, votes, nil
	}
	for i, index := range buckets.Indexes {
		if big.NewInt(0).Cmp(index) == 0 { // back to start
			break
		}
		v, err := types.NewVote(
			time.Unix(buckets.StakeStartTimes[i].Int64(), 0),
			time.Duration(buckets.StakeDurations[i].Uint64()*24)*time.Hour,
			buckets.StakedAmounts[i],
			big.NewInt(0),
			buckets.Owners[i].Bytes(),
			buckets.CanNames[i][:],
			buckets.Decays[i],
		)
		if err != nil {
			return nil, nil, err
		}
		votes = append(votes, v)
		if index.Cmp(previousIndex) > 0 {
			previousIndex = index
		}
	}

	return previousIndex, votes, nil
}

func decodePubKeys(data [][32]byte, num int) ([][]byte, error) {
	if len(data) != 3*num {
		return nil, errors.New("the length of pub key array is not as expected")
	}
	keys := [][]byte{}
	for i := 0; i < num; i++ {
		key := append(data[3*i][:], data[3*i+1][:]...)
		keys = append(keys, append(key, data[3*i+2][0]))
	}

	return keys, nil
}
