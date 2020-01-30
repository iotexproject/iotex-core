// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-election/types"
)

var (
	dummyCaller, _ = address.FromString(address.ZeroAddress)
	// ErrNoData is an error that there's no data in the contract
	ErrNoData = errors.New("no data")
	// ErrEndOfData is an error that reaching end of data in the contract
	ErrEndOfData = errors.New("end of data")
)

type (
	// NativeStaking represents native staking struct
	NativeStaking struct {
		cm              protocol.ChainManager
		getTipBlockTime GetTipBlockTime
		contract        string
		abi             abi.ABI
	}

	pygg struct {
		Count           *big.Int
		Indexes         []*big.Int
		StakeStartTimes []*big.Int
		StakeDurations  []*big.Int
		Decays          []bool
		StakedAmounts   []*big.Int
		CanNames        [][12]byte
		Owners          []common.Address
	}

	// VoteTally is a map of candidates on native chain
	VoteTally struct {
		Candidates map[[12]byte]*state.Candidate
		Buckets    []*types.Bucket
	}
)

// NewNativeStaking creates a NativeStaking instance
func NewNativeStaking(cm protocol.ChainManager, getTipBlockTime GetTipBlockTime) (*NativeStaking, error) {
	abi, err := abi.JSON(strings.NewReader(NsAbi))
	if err != nil {
		return nil, err
	}
	if cm == nil {
		return nil, errors.New("failed to create native staking: empty chain manager")
	}
	if getTipBlockTime == nil {
		return nil, errors.New("failed to create native staking: empty getBlockTime")
	}
	return &NativeStaking{
		cm:              cm,
		getTipBlockTime: getTipBlockTime,
		abi:             abi,
	}, nil
}

// Votes returns the votes on height
func (ns *NativeStaking) Votes(correctGas bool) (*VoteTally, time.Time, error) {
	if ns.contract == "" {
		return nil, time.Time{}, ErrNoData
	}

	now, err := ns.getTipBlockTime()
	if err != nil {
		return nil, time.Time{}, errors.Wrap(err, "failed to get current block time")
	}
	// read voter list from staking contract
	votes := VoteTally{
		Candidates: make(map[[12]byte]*state.Candidate),
		Buckets:    make([]*types.Bucket, 0),
	}
	prevIndex := big.NewInt(0)
	limit := big.NewInt(160)

	for {
		vote, index, err := ns.readBuckets(prevIndex, limit, correctGas)
		log.L().Debug("Read native buckets from contract", zap.Int("size", len(vote)))
		if err == ErrEndOfData {
			// all data been read
			break
		}
		if err != nil {
			log.L().Error(" read native staking contract", zap.Error(err))
			return nil, now, err
		}
		votes.tally(vote, now)
		if len(vote) < int(limit.Int64()) {
			// all data been read
			break
		}
		prevIndex = index
	}
	return &votes, now, nil
}

func (ns *NativeStaking) readBuckets(prevIndx, limit *big.Int, correctGas bool) ([]*types.Bucket, *big.Int, error) {
	data, err := ns.abi.Pack("getActivePyggs", prevIndx, limit)
	if err != nil {
		return nil, nil, err
	}

	// read the staking contract
	gasLimit := uint64(1000000)
	if correctGas {
		gasLimit = 10000000
	}
	ex, err := action.NewExecution(ns.contract, 1, big.NewInt(0), gasLimit, big.NewInt(0), data)
	if err != nil {
		return nil, nil, err
	}
	data, _, err = ns.cm.ExecuteContractRead(dummyCaller, ex)
	if err != nil {
		return nil, nil, err
	}

	// decode the contract read result
	pygg := &pygg{}
	if err = ns.abi.Unpack(pygg, "getActivePyggs", data); err != nil {
		if err.Error() == "abi: unmarshalling empty output" {
			// no data in contract (one possible reason is that contract does not exist yet)
			return nil, nil, ErrNoData
		}
		return nil, nil, err
	}
	if len(pygg.CanNames) == 0 {
		return nil, nil, ErrEndOfData
	}
	buckets := make([]*types.Bucket, len(pygg.CanNames))
	for i := range pygg.CanNames {
		buckets[i], err = types.NewBucket(
			time.Unix(pygg.StakeStartTimes[i].Int64(), 0),
			time.Duration(pygg.StakeDurations[i].Uint64()*24)*time.Hour,
			pygg.StakedAmounts[i],
			pygg.Owners[i].Bytes(),
			pygg.CanNames[i][:],
			pygg.Decays[i],
		)
		if err != nil {
			return nil, nil, err
		}
	}
	// last one of returned indexes should be used as starting index for next query
	return buckets, pygg.Indexes[len(pygg.Indexes)-1], nil
}

// SetContract sets the contract address
func (ns *NativeStaking) SetContract(contract string) {
	if _, err := address.FromString(contract); err != nil {
		zap.S().Panicf("Invalid staking contract %s", contract)
	}
	ns.contract = contract
	zap.S().Infof("Set native staking contract address = %s", contract)
}

func (vt *VoteTally) tally(buckets []*types.Bucket, now time.Time) error {
	for i := range buckets {
		v := buckets[i]
		weighted := types.CalcWeightedVotes(v, now)
		if big.NewInt(0).Cmp(weighted) == 1 {
			return errors.Errorf("weighted amount %s cannot be negative", weighted)
		}
		k := to12Bytes(v.Candidate())
		if c, ok := vt.Candidates[k]; !ok {
			vt.Candidates[k] = &state.Candidate{
				Address:       "",
				Votes:         weighted,
				RewardAddress: "",
				CanName:       v.Candidate(),
			}
		} else {
			// add up the votes
			c.Votes.Add(c.Votes, weighted)
		}
		vt.Buckets = append(vt.Buckets, v)
	}
	return nil
}

func to12Bytes(b []byte) [12]byte {
	var h [12]byte
	if len(b) != 12 {
		panic("invalid CanName: abi stipulates CanName must be [12]byte")
	}
	copy(h[:], b)
	return h
}
