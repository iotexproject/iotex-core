// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-election/types"
)

var (
	// ErrNoData is an error that there's no data in the contract
	ErrNoData = errors.New("no data")
	// ErrEndOfData is an error that reaching end of data in the contract
	ErrEndOfData = errors.New("end of data")
	// ErrWrongData is an error that data is wrong
	ErrWrongData = errors.New("wrong data")
)

type (
	// ReadContract defines a callback function to read contract
	ReadContract func(context.Context, string, []byte, bool) ([]byte, error)
	// NativeStaking represents native staking struct
	NativeStaking struct {
		readContract   ReadContract
		contract       string
		abi            abi.ABI
		bufferEpochNum uint64
		bufferResult   *VoteTally
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
func NewNativeStaking(readContract ReadContract) (*NativeStaking, error) {
	abi, err := abi.JSON(strings.NewReader(NsAbi))
	if err != nil {
		return nil, err
	}

	if readContract == nil {
		return nil, errors.New("failed to create native staking: empty read contract callback")
	}

	return &NativeStaking{
		abi:            abi,
		readContract:   readContract,
		bufferEpochNum: 0,
		bufferResult:   nil,
	}, nil
}

// Votes returns the votes on height
func (ns *NativeStaking) Votes(ctx context.Context, ts time.Time, correctGas bool) (*VoteTally, error) {
	if ns.contract == "" {
		return nil, ErrNoData
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	tipEpochNum := rp.GetEpochNum(bcCtx.Tip.Height)
	if ns.bufferEpochNum == tipEpochNum && ns.bufferResult != nil {
		log.L().Info("Using cache native staking data", zap.Uint64("tip height", bcCtx.Tip.Height))
		return ns.bufferResult, nil
	}
	// read voter list from staking contract
	votes := VoteTally{
		Candidates: make(map[[12]byte]*state.Candidate),
		Buckets:    make([]*types.Bucket, 0),
	}
	prevIndex := big.NewInt(0)
	limit := big.NewInt(256)

	for {
		vote, index, err := ns.readBuckets(ctx, prevIndex, limit, correctGas)
		log.L().Debug("Read native buckets from contract", zap.Int("size", len(vote)))
		if err == ErrEndOfData {
			// all data been read
			break
		}
		if err != nil {
			log.L().Error(" read native staking contract", zap.Error(err))
			return nil, err
		}
		if err = votes.tally(vote, ts); err != nil {
			log.L().Error(" read vote tally", zap.Error(err))
			return nil, err
		}
		if len(vote) < int(limit.Int64()) {
			// all data been read
			break
		}
		prevIndex = index
	}
	ns.bufferEpochNum = tipEpochNum
	ns.bufferResult = &votes

	return &votes, nil
}

func (ns *NativeStaking) readBuckets(ctx context.Context, prevIndx, limit *big.Int, correctGas bool) ([]*types.Bucket, *big.Int, error) {
	data, err := ns.abi.Pack("getActivePyggs", prevIndx, limit)
	if err != nil {
		return nil, nil, err
	}

	data, err = ns.readContract(ctx, ns.contract, data, correctGas)
	if err != nil {
		return nil, nil, err
	}

	// decode the contract read result
	res, err := ns.abi.Unpack("getActivePyggs", data)
	if err != nil {
		if err.Error() == "abi: attempting to unmarshall an empty string while arguments are expected" {
			// no data in contract (one possible reason is that contract does not exist yet)
			return nil, nil, ErrNoData
		}
		return nil, nil, err
	}
	pygg, err := toPgyy(res)
	if err != nil {
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

func toPgyy(v []interface{}) (*pygg, error) {
	// struct pygg has 8 fields
	if len(v) != 8 {
		return nil, ErrWrongData
	}

	c, ok := v[0].(*big.Int)
	if !ok {
		return nil, ErrWrongData
	}
	index, ok := v[1].([]*big.Int)
	if !ok {
		return nil, ErrWrongData
	}
	start, ok := v[2].([]*big.Int)
	if !ok {
		return nil, ErrWrongData
	}
	duration, ok := v[3].([]*big.Int)
	if !ok {
		return nil, ErrWrongData
	}
	decay, ok := v[4].([]bool)
	if !ok {
		return nil, ErrWrongData
	}
	amount, ok := v[5].([]*big.Int)
	if !ok {
		return nil, ErrWrongData
	}
	name, ok := v[6].([][12]byte)
	if !ok {
		return nil, ErrWrongData
	}
	owner, ok := v[7].([]common.Address)
	if !ok {
		return nil, ErrWrongData
	}
	return &pygg{
		Count:           c,
		Indexes:         index,
		StakeStartTimes: start,
		StakeDurations:  duration,
		Decays:          decay,
		StakedAmounts:   amount,
		CanNames:        name,
		Owners:          owner,
	}, nil
}
