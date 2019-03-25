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
	SubscribeNewBlock(chan uint64, chan error, chan bool)
	// TipHeight returns the latest height
	TipHeight() (uint64, error)
	// Candidates returns the candidates on height
	Candidates(uint64, *big.Int, uint8) (*big.Int, []*types.Candidate, error)
	// Votes returns the votes on height
	Votes(uint64, *big.Int, uint8) (*big.Int, []*types.Vote, error)
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
	clientURLs []string,
	registerContractAddress common.Address,
	stakingContractAddress common.Address,
) (Carrier, error) {
	if len(clientURLs) == 0 {
		return nil, errors.New("client URL list is empty")
	}
	var client *ethclient.Client
	var err error
	var currentClientURLIndex int
	for currentClientURLIndex = 0; currentClientURLIndex < len(clientURLs); currentClientURLIndex++ {
		client, err = ethclient.Dial(clientURLs[currentClientURLIndex])
		if err == nil {
			break
		}
		zap.L().Error(
			"client is not reachable",
			zap.String("url", clientURLs[currentClientURLIndex]),
			zap.Error(err),
		)
	}
	if err != nil {
		return nil, err
	}
	return &ethereumCarrier{
		client:                  client,
		currentClientURLIndex:   currentClientURLIndex,
		clientURLs:              clientURLs,
		stakingContractAddress:  stakingContractAddress,
		registerContractAddress: registerContractAddress,
	}, nil
}

func (evc *ethereumCarrier) Close() {
	evc.client.Close()
}

func (evc *ethereumCarrier) BlockTimestamp(height uint64) (ts time.Time, err error) {
	var header *ethtypes.Header
	for i := 0; i < len(evc.clientURLs); i++ {
		if header, err = evc.client.HeaderByNumber(
			context.Background(),
			big.NewInt(0).SetUint64(height),
		); err == nil {
			ts = time.Unix(header.Time.Int64(), 0)
			return
		}
		var rotated bool
		if rotated, err = evc.rotateClient(err); !rotated && err != nil {
			return
		}
	}
	return ts, errors.New("failed to get block timestamp")
}

func (evc *ethereumCarrier) SubscribeNewBlock(height chan uint64, report chan error, unsubscribe chan bool) {
	ticker := time.NewTicker(60 * time.Second)
	lastHeight := uint64(0)
	go func() {
		for {
			select {
			case <-unsubscribe:
				unsubscribe <- true
				return
			case <-ticker.C:
				if tipHeight, err := evc.tipHeight(lastHeight); err != nil {
					report <- err
				} else {
					height <- tipHeight
				}
			}
		}
	}()
}

func (evc *ethereumCarrier) TipHeight() (uint64, error) {
	return evc.tipHeight(0)
}

func (evc *ethereumCarrier) tipHeight(lastHeight uint64) (uint64, error) {
	for i := 0; i < len(evc.clientURLs); i++ {
		header, err := evc.client.HeaderByNumber(context.Background(), nil)
		if err == nil {
			if header.Number.Uint64() > lastHeight {
				return header.Number.Uint64(), nil
			}
			zap.L().Warn(
				"current client is out of date",
				zap.String("url", evc.clientURLs[evc.currentClientURLIndex]),
				zap.Uint64("clientHeight", header.Number.Uint64()),
				zap.Uint64("lastHeight", lastHeight),
			)
		}
		if rotated, err := evc.rotateClient(err); !rotated && err != nil {
			return 0, err
		}
	}
	return 0, errors.New("failed to get tip height")
}

func (evc *ethereumCarrier) candidates(
	opts *bind.CallOpts,
	startIndex *big.Int,
	limit *big.Int,
) (result struct {
	Names          [][12]byte
	Addresses      []common.Address
	IoOperatorAddr [][32]byte
	IoRewardAddr   [][32]byte
	Weights        []*big.Int
}, err error) {
	var caller *contract.RegisterCaller
	for i := 0; i < len(evc.clientURLs); i++ {
		if caller, err = contract.NewRegisterCaller(evc.registerContractAddress, evc.client); err == nil {
			var count *big.Int
			if count, err = caller.CandidateCount(opts); err != nil {
				return
			}
			if startIndex.Cmp(count) >= 0 {
				return
			}
			if result, err = caller.GetAllCandidates(opts, startIndex, limit); err == nil {
				return
			}
		}
		var rotated bool
		if rotated, err = evc.rotateClient(err); !rotated && err != nil {
			return
		}
	}
	return result, errors.New("failed to get candidates")
}

func (evc *ethereumCarrier) Candidates(
	height uint64,
	startIndex *big.Int,
	count uint8,
) (*big.Int, []*types.Candidate, error) {
	if startIndex == nil || startIndex.Cmp(big.NewInt(1)) < 0 {
		startIndex = big.NewInt(1)
	}
	retval, err := evc.candidates(
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

// EthereumBucketsResult defines the data structure the buckets api returns
type EthereumBucketsResult struct {
	Count           *big.Int
	Indexes         []*big.Int
	StakeStartTimes []*big.Int
	StakeDurations  []*big.Int
	Decays          []bool
	StakedAmounts   []*big.Int
	CanNames        [][12]byte
	Owners          []common.Address
}

func (evc *ethereumCarrier) buckets(
	opts *bind.CallOpts,
	previousIndex *big.Int,
	limit *big.Int,
) (result EthereumBucketsResult, err error) {
	var caller *contract.StakingCaller
	for i := 0; i < len(evc.clientURLs); i++ {
		if caller, err = contract.NewStakingCaller(evc.stakingContractAddress, evc.client); err == nil {
			var bucket struct {
				CanName          [12]byte
				StakedAmount     *big.Int
				StakeDuration    *big.Int
				StakeStartTime   *big.Int
				NonDecay         bool
				UnstakeStartTime *big.Int
				BucketOwner      common.Address
				CreateTime       *big.Int
				Prev             *big.Int
				Next             *big.Int
			}
			if bucket, err = caller.Buckets(opts, previousIndex); err == nil {
				if bucket.Next.Cmp(big.NewInt(0)) <= 0 {
					return
				}
				if result, err = caller.GetActiveBuckets(opts, previousIndex, limit); err == nil {
					return
				}
			}
		}
		var rotated bool
		if rotated, err = evc.rotateClient(err); !rotated && err != nil {
			return
		}
	}
	return result, errors.New("failed to get votes")
}

func (evc *ethereumCarrier) Votes(
	height uint64,
	previousIndex *big.Int,
	count uint8,
) (*big.Int, []*types.Vote, error) {
	if previousIndex == nil || previousIndex.Cmp(big.NewInt(0)) < 0 {
		previousIndex = big.NewInt(0)
	}
	buckets, err := evc.buckets(
		&bind.CallOpts{BlockNumber: new(big.Int).SetUint64(height)},
		previousIndex,
		big.NewInt(int64(count)),
	)
	if err != nil {
		return nil, nil, err
	}
	votes := []*types.Vote{}
	if buckets.Count == nil || buckets.Count.Cmp(big.NewInt(0)) == 0 || len(buckets.Indexes) == 0 {
		return previousIndex, votes, nil
	}
	for i, index := range buckets.Indexes {
		if big.NewInt(0).Cmp(index) == 0 { // back to start, this is a redundant condition
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

func (evc *ethereumCarrier) rotateClient(cause error) (rotated bool, err error) {
	if cause != nil {
		switch cause.Error() {
		case "tls: use of closed connection":
			zap.L().Error("connection closed", zap.Error(cause))
		case "EOF":
			zap.L().Error("connection error", zap.Error(cause))
		case "no suitable peers available":
			zap.L().Error("node out of date", zap.Error(cause))
		default:
			return false, cause
		}
	}
	evc.currentClientURLIndex = (evc.currentClientURLIndex + 1) % len(evc.clientURLs)
	zap.L().Info("rotate to new client", zap.String("url", evc.clientURLs[evc.currentClientURLIndex]))
	var client *ethclient.Client
	if client, err = ethclient.Dial(evc.clientURLs[evc.currentClientURLIndex]); err != nil {
		return true, err
	}
	evc.client.Close()
	evc.client = client
	return true, nil
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
