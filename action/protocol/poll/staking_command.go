// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/state"
)

type stakingCommand struct {
	addr        address.Address
	hu          config.HeightUpgrade
	stakingV1   Protocol
	stakingV2   *staking.Protocol
	candIndexer *CandidateIndexer
}

// NewStakingCommand creates a staking command center to manage staking committee and new native staking
func NewStakingCommand(
	hu config.HeightUpgrade,
	candIndexer *CandidateIndexer,
	stkV1 Protocol,
	stkV2 *staking.Protocol,
) (Protocol, error) {
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}

	sc := stakingCommand{
		hu:          hu,
		addr:        addr,
		stakingV1:   stkV1,
		stakingV2:   stkV2,
		candIndexer: candIndexer,
	}

	if stkV1 == nil && stkV2 == nil {
		return nil, errors.New("empty staking protocol")
	}
	return &sc, nil
}

func (sc *stakingCommand) CreateGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	if sc.stakingV1 != nil {
		if err := sc.stakingV1.CreateGenesisStates(ctx, sm); err != nil {
			return err
		}
	}

	if sc.stakingV2 != nil {
		if err := sc.stakingV2.CreateGenesisStates(ctx, sm); err != nil {
			return err
		}
	}
	return nil
}

func (sc *stakingCommand) Start(ctx context.Context) error {
	if sc.stakingV1 != nil {
		if starter, ok := sc.stakingV1.(lifecycle.Starter); ok {
			return starter.Start(ctx)
		}
	}
	return nil
}

func (sc *stakingCommand) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	// TODO: handle V2
	return sc.stakingV1.CreateGenesisStates(ctx, sm)
}

func (sc *stakingCommand) CreatePostSystemActions(ctx context.Context) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, sc)
}

func (sc *stakingCommand) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	if sc.stakingV1 == nil {
		return handle(ctx, act, sm, sc.candIndexer, sc.addr.String())
	}

	if sc.stakingV2 == nil {
		return sc.stakingV1.Handle(ctx, act, sm)
	}

	// transition to V2 starting Fairbank
	height, err := sm.Height()
	if err != nil {
		return nil, err
	}
	if sc.hu.IsPost(config.Fairbank, height) {
		return handle(ctx, act, sm, sc.candIndexer, sc.addr.String())
	}
	return sc.stakingV1.Handle(ctx, act, sm)
}

func (sc *stakingCommand) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, sc, act)
}

func (sc *stakingCommand) CalculateCandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	if sc.stakingV1 == nil {
		return sc.stakingV2.ActiveCandidates(ctx)
	}

	if sc.stakingV2 == nil {
		return sc.stakingV1.CalculateCandidatesByHeight(ctx, height)
	}

	// transition to V2 starting Fairbank
	if sc.hu.IsPost(config.Fairbank, height) {
		return sc.stakingV2.ActiveCandidates(ctx)
	}
	return sc.stakingV1.CalculateCandidatesByHeight(ctx, height)
}

// Delegates returns exact number of delegates of current epoch
func (sc *stakingCommand) Delegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	// TODO: handle V2
	return sc.stakingV1.Delegates(ctx, sr)
}

// NextDelegates returns exact number of delegates of next epoch
func (sc *stakingCommand) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	// TODO: handle V2
	return sc.stakingV1.Delegates(ctx, sr)
}

// Candidates returns candidate list from state factory of current epoch
func (sc *stakingCommand) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	// TODO: handle V2
	return sc.stakingV1.Candidates(ctx, sr)
}

// NextCandidates returns candidate list from state factory of next epoch
func (sc *stakingCommand) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	// TODO: handle V2
	return sc.stakingV1.NextCandidates(ctx, sr)
}

func (sc *stakingCommand) ReadState(ctx context.Context, sr protocol.StateReader, method []byte, args ...[]byte) ([]byte, error) {
	// TODO: handle V2
	return sc.stakingV1.ReadState(ctx, sr, method, args...)
}

// Register registers the protocol with a unique ID
func (sc *stakingCommand) Register(r *protocol.Registry) error {
	return r.Register(protocolID, sc)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (sc *stakingCommand) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, sc)
}
