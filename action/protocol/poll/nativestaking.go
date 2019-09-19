// Copyright (c) 2019 IoTeX Foundation
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
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/types"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
)

var (
	dummyCaller, _ = address.FromString(address.ZeroAddress)

	// ErrNoData is an error that there's no data in the contract
	ErrNoData = errors.New("no data")
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
	VoteTally map[[12]byte]*state.Candidate
)

// NewNativeStaking creates a NativeStaking instance
func NewNativeStaking(cm protocol.ChainManager, getTipBlockTime GetTipBlockTime, staking string) (*NativeStaking, error) {
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
	if _, err := address.FromString(staking); err != nil {
		return nil, errors.Errorf("invalid staking contract %s", staking)
	}
	return &NativeStaking{cm, getTipBlockTime, staking, abi}, nil
}

// Votes returns the votes on height
func (ns *NativeStaking) Votes(height uint64) (VoteTally, error) {
	if ns.contract == "" {
		return nil, ErrNoData
	}

	now, err := ns.getTipBlockTime()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current block time")
	}
	// read voter list from staking contract
	votes := make(VoteTally)
	prevIndex := big.NewInt(0)
	limit := big.NewInt(256)

	for {
		vote, err := ns.readBuckets(prevIndex, limit)
		if err == ErrNoData {
			// all data been read
			break
		}
		if err != nil {
			return nil, err
		}
		votes.tally(vote, now)
		if len(vote) < int(limit.Int64()) {
			break
		}
		prevIndex.Add(prevIndex, limit)
	}
	return votes, nil
}

func (ns *NativeStaking) readBuckets(prevIndx, limit *big.Int) ([]*types.Bucket, error) {
	data, err := ns.abi.Pack("getActivePyggs", prevIndx, limit)
	if err != nil {
		return nil, err
	}

	// read the staking contract
	ex, err := action.NewExecution(ns.contract, 1, big.NewInt(0), 1000000, big.NewInt(0), data)
	if err != nil {
		return nil, err
	}
	data, _, err = ns.cm.ExecuteContractRead(dummyCaller, ex)
	if err != nil {
		return nil, err
	}

	// decode the contract read result
	pygg := &pygg{}
	if err := ns.abi.Unpack(pygg, "getActivePyggs", data); err != nil {
		return nil, err
	}
	if len(pygg.CanNames) == 0 {
		return nil, ErrNoData
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
			return nil, err
		}
	}
	return buckets, nil
}

func (vt VoteTally) tally(buckets []*types.Bucket, now time.Time) error {
	for i := range buckets {
		v := buckets[i]
		weighted := types.CalcWeightedVotes(v, now)
		if big.NewInt(0).Cmp(weighted) == 1 {
			return errors.Errorf("weighted amount %s cannot be negative", weighted)
		}
		k := to12Bytes(v.Candidate())
		if c, ok := vt[k]; !ok {
			vt[k] = &state.Candidate{"", weighted, "", v.Candidate()}
		} else {
			// add up the votes
			c.Votes.Add(c.Votes, weighted)
		}
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
