// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/util"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/state"
)

type stakingCommand struct {
	addr           address.Address
	stakingV1      Protocol
	stakingV2      *staking.Protocol
	candIndexer    *CandidateIndexer
	slasher        *Slasher
	scoreThreshold *big.Int
}

// NewStakingCommand creates a staking command center to manage staking committee and new native staking
func NewStakingCommand(
	candIndexer *CandidateIndexer,
	sh *Slasher,
	scoreThreshold *big.Int,
	stkV1 Protocol,
	stkV2 *staking.Protocol,
) (Protocol, error) {
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		return nil, err
	}

	sc := stakingCommand{
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
	// if v1 exists, bootstrap from v1 only
	if sc.stakingV1 != nil {
		return sc.stakingV1.CreateGenesisStates(ctx, sm)
	}

	cands, err := sc.stakingV2.ActiveCandidates(ctx)
	if err != nil {
		return err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	cands = sc.filterAndSortCandidatesByVoteScore(cands, bcCtx.Tip.Timestamp)
	return setCandidates(ctx, sm, sc.candIndexer, cands, uint64(1))
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
	return sc.slasher.CreatePreStates(ctx, sm, sc.candIndexer)
}

func (sc *stakingCommand) CreatePostSystemActions(ctx context.Context) ([]action.Envelope, error) {
	return createPostSystemActions(ctx, sc)
}

func (sc *stakingCommand) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// transition to V2 starting Fairbank
	height, err := sm.Height()
	if err != nil {
		return nil, err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	if sc.stakingV1 == nil || hu.IsPost(config.Fairbank, height) {
		return handle(ctx, act, sm, sc.candIndexer, sc.addr.String())
	}
	return sc.stakingV1.Handle(ctx, act, sm)
}

func (sc *stakingCommand) Validate(ctx context.Context, act action.Action) error {
	return validate(ctx, sc, act)
}

func (sc *stakingCommand) CalculateCandidatesByHeight(ctx context.Context, height uint64) (state.CandidateList, error) {
	// transition to V2 starting Fairbank
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	if sc.stakingV1 == nil || hu.IsPost(config.Fairbank, height) {
		cands, err := sc.stakingV2.ActiveCandidates(ctx)
		if err != nil {
			return cands, err
		}
		bcCtx := protocol.MustGetBlockchainCtx(ctx)
		return sc.filterAndSortCandidatesByVoteScore(cands, bcCtx.Tip.Timestamp), nil
	}
	return sc.stakingV1.CalculateCandidatesByHeight(ctx, height)
}

// Delegates returns exact number of delegates of current epoch
func (sc *stakingCommand) Delegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return sc.slasher.GetActiveBlockProducers(ctx, sr, false)
}

// NextDelegates returns exact number of delegates of next epoch
func (sc *stakingCommand) NextDelegates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return sc.slasher.GetActiveBlockProducers(ctx, sr, true)
}

// Candidates returns candidate list from state factory of current epoch
func (sc *stakingCommand) Candidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return sc.slasher.GetCandidates(ctx, sr, false)
}

// NextCandidates returns candidate list from state factory of next epoch
func (sc *stakingCommand) NextCandidates(ctx context.Context, sr protocol.StateReader) (state.CandidateList, error) {
	return sc.slasher.GetCandidates(ctx, sr, true)
}

func (sc *stakingCommand) ReadState(ctx context.Context, sr protocol.StateReader, method []byte, args ...[]byte) ([]byte, error) {
	if sc.stakingV1 == nil {
		return sc.slasher.ReadState(ctx, sr, sc.candIndexer, method, args...)
	}
	height, err := sr.Height()
	if err != nil {
		return nil, err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	if hu.IsPost(config.Fairbank, height) {
		res, err := sc.slasher.ReadState(ctx, sr, sc.candIndexer, method, args...)
		if err == nil {
			return res, nil
		}
	}
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

func (sc *stakingCommand) filterAndSortCandidatesByVoteScore(list state.CandidateList, ts time.Time) state.CandidateList {
	candidates := make(map[string]*state.Candidate)
	candidateScores := make(map[string]*big.Int)
	for _, cand := range list {
		if cand.Votes.Cmp(sc.scoreThreshold) >= 0 {
			clone := cand.Clone()
			candidates[string(clone.CanName)] = clone
			candidateScores[string(clone.CanName)] = clone.Votes
		}
	}
	sorted := util.Sort(candidateScores, uint64(ts.Unix()))
	res := make(state.CandidateList, 0, len(sorted))
	for _, name := range sorted {
		res = append(res, candidates[name])
	}
	return res
}
