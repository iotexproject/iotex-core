// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
)

type stakingCommittee struct {
	hu                config.HeightUpgrade
	getEpochHeight    GetEpochHeight
	getEpochNum       GetEpochNum
	governanceStaking Protocol
	nativeStaking     *NativeStaking
	scoreThreshold    *big.Int
}

// NewStakingCommittee creates a staking committee which fetch result from governance chain and native staking
func NewStakingCommittee(
	hu config.HeightUpgrade,
	gs Protocol,
	cm protocol.ChainManager,
	getTipBlockTime GetTipBlockTime,
	getEpochHeight GetEpochHeight,
	getEpochNum GetEpochNum,
	staking string,
	scoreThreshold *big.Int,
) (Protocol, error) {
	if getEpochHeight == nil {
		return nil, errors.New("failed to create native staking: empty getEpochHeight")
	}
	if getEpochNum == nil {
		return nil, errors.New("failed to create native staking: empty getEpochNum")
	}
	var ns *NativeStaking
	if staking != "" {
		var err error
		if ns, err = NewNativeStaking(cm, getTipBlockTime, staking); err != nil {
			return nil, errors.New("failed to create native staking")
		}
	}
	return &stakingCommittee{
		hu:                hu,
		governanceStaking: gs,
		nativeStaking:     ns,
		getEpochHeight:    getEpochHeight,
		getEpochNum:       getEpochNum,
		scoreThreshold:    scoreThreshold,
	}, nil
}

func (sc *stakingCommittee) Initialize(ctx context.Context, sm protocol.StateManager) error {
	return sc.governanceStaking.Initialize(ctx, sm)
}

func (sc *stakingCommittee) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return sc.governanceStaking.Handle(ctx, act, sm)
}

func (sc *stakingCommittee) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, sc, act)
}

func (sc *stakingCommittee) DelegatesByHeight(height uint64) (state.CandidateList, error) {
	cand, err := sc.governanceStaking.DelegatesByHeight(height)
	if err != nil {
		return nil, err
	}
	// convert to epoch start height
	epochHeight := sc.getEpochHeight(sc.getEpochNum(height))
	if sc.hu.IsPre(config.Bering, epochHeight) {
		return cand, nil
	}
	// native staking starts from Bering
	if sc.nativeStaking == nil {
		return nil, errors.New("native staking was not set after bering height")
	}
	nativeVotes, err := sc.nativeStaking.Votes()
	if err == ErrNoData {
		// no native staking data
		return cand, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get native chain candidates")
	}
	return sc.mergeDelegates(cand, nativeVotes), nil
}

func (sc *stakingCommittee) ReadState(ctx context.Context, sm protocol.StateManager, method []byte, args ...[]byte) ([]byte, error) {
	return sc.governanceStaking.ReadState(ctx, sm, method, args...)
}

func (sc *stakingCommittee) mergeDelegates(list state.CandidateList, votes VoteTally) state.CandidateList {
	// as of now, native staking does not have register contract, only voting/staking contract
	// it is assumed that all votes done on native staking target for delegates registered on Ethereum
	// votes cast to all outside address will not be counted and simply ignored
	var merged state.CandidateList
	for _, cand := range list {
		clone := cand.Clone()
		name := to12Bytes(clone.CanName)
		if v, ok := votes[name]; ok {
			clone.Votes.Add(clone.Votes, v.Votes)
		}
		if clone.Votes.Cmp(sc.scoreThreshold) >= 0 {
			merged = append(merged, clone)
		}
	}
	return merged
}
