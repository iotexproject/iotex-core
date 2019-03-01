// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package carrier

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"

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
	// RotateClientURL rotates client URL index and sets new client
	RotateClient() error
	// Close closes carrier
	Close()
}

type ethereumCarrier struct {
	client                  *ethclient.Client
	currentClientURLIndex   int
	clientURLs              []string
	stakingContractAddress  common.Address
	registerContractAddress common.Address
}

// NewEthereumVoteCarrier defines a carrier to fetch votes from ethereum contract
func NewEthereumVoteCarrier(
	clientURLs              []string,
	registerContractAddress common.Address,
	stakingContractAddress common.Address,
) (Carrier, error) {
	if len(clientURLs) == 0 {
		return nil, errors.New("client URL list is empty")
	}
	client, err := ethclient.Dial(clientURLs[0])
	if err != nil {
		return nil, err
	}
	return &ethereumCarrier{
		client:                  client,
		currentClientURLIndex:   0,
		clientURLs:              clientURLs,
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
		return time.Now(), evc.redial(err)
	}
	return time.Unix(header.Time.Int64(), 0), nil
}

func (evc *ethereumCarrier) SubscribeNewBlock(cb func(uint64), close chan bool) error {
	headers := make(chan *ethtypes.Header)
	sub, err := evc.client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		return evc.redial(err)
	}
	go func() {
		for {
			select {
			case closed := <-close:
				close <- closed
				return
			case err := <-sub.Err():
				for {
					if err := evc.rotateClient(); err != nil {
						zap.L().Error("failed to rotate client", zap.Error(err))
					}
					if sub, err = evc.client.SubscribeNewHead(context.Background(), headers); err == nil {
						break
					}
					zap.L().Error("failed to resubscribe new head", zap.Error(err))
				}
			case header := <-headers:
				zap.L().Debug("New ethereum block", zap.Uint64("height", header.Number.Uint64()))
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
		return nil, nil, evc.redial(err)
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
	operatorPubKeys, err := decodeAddress(retval.IoOperatorAddr, num)
	if err != nil {
		return nil, nil, err
	}
	rewardPubKeys, err := decodeAddress(retval.IoRewardAddr, num)
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
			retval.Weights[i].Uint64(),
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
		return nil, nil, evc.redial(err)
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

func (evc *ethereumCarrier) RotateClient() error {
	return evc.rotateClient()
}

func (evc *ethereumCarrier) redial(err error) error {
	switch err.Error() {
	case "tls: use of closed connection":
		fallthrough
	case "EOF":
		zap.L().Warn("reset ethclient", zap.Error(err))
		evc.client.Close()
		var newErr error
		if evc.client, newErr = ethclient.Dial(evc.clientURLs[evc.currentClientURLIndex]); newErr != nil {
			err = newErr
		}
	}

	return err
}

func (evc *ethereumCarrier) rotateClient() error {
	evc.currentClientURLIndex++
	evc.currentClientURLIndex = (evc.currentClientURLIndex + 1) % len(evc.clientURLs)
	evc.client.Close()
	var err error
	if evc.client, err = ethclient.Dial(evc.clientURLs[evc.currentClientURLIndex]); err != nil {
		return err
	}
	return nil
}

func decodeAddress(data [][32]byte, num int) ([][]byte, error) {
	if len(data) != 2*num {
		return nil, errors.New("the length of address array is not as expected")
	}
	keys := [][]byte{}
	for i := 0; i < num; i++ {
		key := append(data[2*i][:], data[2*i+1][:9]...)
		keys = append(keys, key)
	}

	return keys, nil
}
