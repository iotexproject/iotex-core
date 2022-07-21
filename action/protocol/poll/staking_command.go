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
	"github.com/iotexproject/iotex-core/state"
)

type stakingCommand struct {
	addr      address.Address
	stakingV1 Protocol
	stakingV2 Protocol
}

// NewStakingCommand creates a staking command center to manage staking committee and new native staking
func NewStakingCommand(stkV1 Protocol, stkV2 Protocol) (Protocol, error) {
	if stkV1 == nil && stkV2 == nil {
		return nil, errors.New("empty staking protocol")
	}

	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}

	return &stakingCommand{
		addr:      addr,
		stakingV1: stkV1,
		stakingV2: stkV2,
	}, nil
}

func (sc *stakingCommand) CreateGenesisStates(ctx context.Context, sm protocol.StateManager) error {
	// if v1 exists, bootstrap from v1 only
	if sc.stakingV1 != nil {
		return sc.stakingV1.CreateGenesisStates(ctx, sm)
	}
	return sc.stakingV2.CreateGenesisStates(ctx, sm)
}

func (sc *stakingCommand) Start(ctx context.Context, sr protocol.StateReader) (interface{}, error) {
	if sc.stakingV1 != nil {
		if starter, ok := sc.stakingV1.(protocol.Starter); ok {
			if _, err := starter.Start(ctx, sr); err != nil {
				return nil, err
			}
		}
	}

	if sc.stakingV2 != nil {
		if starter, ok := sc.stakingV2.(protocol.Starter); ok {
			return starter.Start(ctx, sr)
		}
	}
	return nil, nil
}

func (sc *stakingCommand) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	if sc.useV2(ctx, sm) {
		if p, ok := sc.stakingV2.(protocol.PreStatesCreator); ok {
			return p.CreatePreStates(ctx, sm)
		}
	}
	if p, ok := sc.stakingV1.(protocol.PreStatesCreator); ok {
		return p.CreatePreStates(ctx, sm)
	}
	return nil
}

func (sc *stakingCommand) CreatePostSystemActions(ctx context.Context, sr protocol.StateReader) ([]action.Envelope, error) {
	// no height here,  v1 v2 has the same createPostSystemActions method, so directly use common one
	return createPostSystemActions(ctx, sr, sc)
}

func (sc *stakingCommand) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	if sc.useV2(ctx, sm) {
		return sc.stakingV2.Handle(ctx, act, sm)
	}
	return sc.stakingV1.Handle(ctx, act, sm)
}

func (sc *stakingCommand) Validate(ctx context.Context, act action.Action, sr protocol.StateReader) error {
	// no height here,  v1 v2 has the same validate method, so directly use common one
	return validate(ctx, sr, sc, act)
}

func (sc *stakingCommand) CalculateCandidatesByHeight(ctx context.Context, sr protocol.StateReader, height uint64) (state.CandidateList, error) {
	if sc.useV2ByHeight(ctx, height) {
		return sc.stakingV2.CalculateCandidatesByHeight(ctx, sr, height)
	}
	return sc.stakingV1.CalculateCandidatesByHeight(ctx, sr, height)
}

func (sc *stakingCommand) CalculateUnproductiveDelegates(
	ctx context.Context,
	sr protocol.StateReader,
) ([]string, error) {
	if sc.useV2(ctx, sr) {
		return sc.stakingV2.CalculateUnproductiveDelegates(ctx, sr)
	}
	return sc.stakingV1.CalculateUnproductiveDelegates(ctx, sr)
}

// Delegates returns exact number of delegates of current epoch
func (sc *stakingCommand) Delegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	if sc.useV2(ctx, sr) {
		return sc.stakingV2.Delegates(ctx, sr)
	}
	return sc.stakingV1.Delegates(ctx, sr)
}

// NextDelegates returns exact number of delegates of next epoch
func (sc *stakingCommand) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	if sc.useV2(ctx, sr) {
		return sc.stakingV2.NextDelegates(ctx, sr)
	}
	return sc.stakingV1.NextDelegates(ctx, sr)
}

// Candidates returns candidate list from state factory of current epoch
func (sc *stakingCommand) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	if sc.useV2(ctx, sr) {
		return sc.stakingV2.Candidates(ctx, sr)
	}
	return sc.stakingV1.Candidates(ctx, sr)
}

// NextCandidates returns candidate list from state factory of next epoch
func (sc *stakingCommand) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	if sc.useV2(ctx, sr) {
		return sc.stakingV2.NextCandidates(ctx, sr)
	}
	return sc.stakingV1.NextCandidates(ctx, sr)
}

func (sc *stakingCommand) ReadState(ctx context.Context, sr protocol.StateReader, method []byte, args ...[]byte) ([]byte, uint64, error) {
	if sc.useV2(ctx, sr) {
		res, height, err := sc.stakingV2.ReadState(ctx, sr, method, args...)
		if err != nil && sc.stakingV1 != nil {
			// check if reading from v1 only method
			return sc.stakingV1.ReadState(ctx, sr, method, args...)
		}
		return res, height, nil
	}
	return sc.stakingV1.ReadState(ctx, sr, method, args...)
}

// Register registers the protocol with a unique ID
func (sc *stakingCommand) Register(r *protocol.Registry) error {
	return r.Register(_protocolID, sc)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (sc *stakingCommand) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(_protocolID, sc)
}

func (sc *stakingCommand) Name() string {
	return _protocolID
}

func (sc *stakingCommand) useV2(ctx context.Context, sr protocol.StateReader) bool {
	height, err := sr.Height()
	if err != nil {
		panic("failed to return out height from state reader")
	}
	return sc.useV2ByHeight(ctx, height)
}

func (sc *stakingCommand) useV2ByHeight(ctx context.Context, height uint64) bool {
	featureCtx := protocol.MustGetFeatureWithHeightCtx(ctx)
	if sc.stakingV1 == nil || featureCtx.UseV2Staking(height) {
		return true
	}
	return false
}
