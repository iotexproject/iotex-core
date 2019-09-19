// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
)

type stakingCommittee struct {
	hu                config.HeightUpgrade
	governanceStaking Protocol
	nativeStaking     *NativeStaking
}

// NewStakingCommittee creates a staking committee which fetch result from governance chain and native staking
func NewStakingCommittee(
	hu config.HeightUpgrade,
	gs Protocol,
	cm protocol.ChainManager,
	getTipBlockTime GetTipBlockTime,
	staking string,
) (Protocol, error) {
	var ns *NativeStaking
	if staking != "" {
		var err error
		if ns, err = NewNativeStaking(cm, getTipBlockTime, staking); err != nil {
			return nil, errors.New("failed to create native staking")
		}
	}
	return &stakingCommittee{
		hu: hu,
		governanceStaking: gs,
		nativeStaking: ns,
	}, nil
}

func (sc *stakingCommittee) Initialize(ctx context.Context, sm protocol.StateManager) error {
	return sc.governanceStaking.Initialize(ctx, sm)
}

func (sc *stakingCommittee) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	return sc.governanceStaking.Handle(ctx, act, sm)
}

func (sc *stakingCommittee) Validate(ctx context.Context, act action.Action) error {
	return sc.governanceStaking.Validate(ctx, act)
}

func (sc *stakingCommittee) DelegatesByHeight(height uint64) (state.CandidateList, error) {
	cand, err := sc.governanceStaking.DelegatesByHeight(height)
	if err != nil {
		return nil, err
	}
	// native staking starts from Bering
	if sc.nativeStaking == nil || sc.hu.IsPre(config.Bering, height) {
		return cand, nil
	}
	// as of now, native staking result does not respect height (will be in the future)
	nativeVotes, err := sc.nativeStaking.Votes(height)
	if err == ErrNoData {
		// no native staking data
		return cand, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get native chain candidates")
	}
	// merge the candidates
	if err := sc.mergeDelegates(cand, nativeVotes); err != nil {
		return nil, errors.Wrap(err, "failed to merge candidates")
	}
	return cand, nil
}

func (sc *stakingCommittee) ReadState(ctx context.Context, sm protocol.StateManager, method []byte, args ...[]byte) ([]byte, error) {
	return sc.governanceStaking.ReadState(ctx, sm, method, args...)
}

func (sc *stakingCommittee) mergeDelegates(cand state.CandidateList, votes VoteTally) error {
	// as of now, native staking does not have register contract, only voting/staking contract
	// it is assumed that all votes done on native staking target for delegates registered on Ethereum
	// votes cast to all outside address will not be counted and simply ignored
	for i, c := range cand {
		name := to12Bytes(c.CanName)
		if v, ok := votes[name]; ok {
			cand[i].Votes.Add(c.Votes, v.Votes)
		}
	}
	return nil
}
