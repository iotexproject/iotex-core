// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

func validateDelegates(cs state.CandidateList) error {
	zero := big.NewInt(0)
	addrs := map[string]bool{}
	lastVotes := zero
	for _, candidate := range cs {
		if _, exists := addrs[candidate.Address]; exists {
			return errors.Errorf("duplicate candidate %s", candidate.Address)
		}
		addrs[candidate.Address] = true
		if candidate.Votes.Cmp(zero) < 0 {
			return errors.New("votes for candidate cannot be negative")
		}
		if lastVotes.Cmp(zero) > 0 && lastVotes.Cmp(candidate.Votes) < 0 {
			return errors.New("candidate list is not sorted")
		}
	}
	return nil
}

func handle(ctx context.Context, act action.Action, sm protocol.StateManager, protocolAddr string) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	r, ok := act.(*action.PutPollResult)
	if !ok {
		return nil, nil
	}
	zap.L().Debug("Handle PutPollResult Action", zap.Uint64("height", r.Height()))

	if err := setCandidates(sm, r.Candidates(), r.Height()); err != nil {
		return nil, errors.Wrap(err, "failed to set candidates")
	}
	return &action.Receipt{
		Status:          uint64(iotextypes.ReceiptStatus_Success),
		ActionHash:      actionCtx.ActionHash,
		BlockHeight:     blkCtx.BlockHeight,
		GasConsumed:     actionCtx.IntrinsicGas,
		ContractAddress: protocolAddr,
	}, nil
}

func validate(ctx context.Context, p Protocol, act action.Action) error {
	ppr, ok := act.(*action.PutPollResult)
	if !ok {
		return nil
	}
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	if blkCtx.Producer.String() != actionCtx.Caller.String() {
		return errors.New("Only producer could create this protocol")
	}
	proposedDelegates := ppr.Candidates()
	if err := validateDelegates(proposedDelegates); err != nil {
		return err
	}
	ds, err := p.DelegatesByHeight(ctx, blkCtx.BlockHeight)
	if err != nil {
		return err
	}
	if len(ds) != len(proposedDelegates) {
		msg := fmt.Sprintf(", %d, is not as expected, %d",
			len(proposedDelegates),
			len(ds))
		return errors.Wrap(ErrProposedDelegatesLength, msg)
	}
	for i, d := range ds {
		if !proposedDelegates[i].Equal(d) {
			msg := fmt.Sprintf(", %v vs %v (expected)",
				proposedDelegates,
				ds)
			return errors.Wrap(ErrDelegatesNotAsExpected, msg)
		}
	}
	return nil
}

// setCandidates sets the candidates for the given state manager
func setCandidates(
	sm protocol.StateManager,
	candidates state.CandidateList,
	height uint64,
) error {
	for _, candidate := range candidates {
		delegate, err := accountutil.LoadOrCreateAccount(sm, candidate.Address)
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account for delegate %s", candidate.Address)
		}
		delegate.IsCandidate = true
		if err := candidatesutil.LoadAndAddCandidates(sm, height, candidate.Address); err != nil {
			return err
		}
		if err := accountutil.StoreAccount(sm, candidate.Address, delegate); err != nil {
			return errors.Wrap(err, "failed to update pending account changes to trie")
		}
		log.L().Debug(
			"add candidate",
			zap.String("address", candidate.Address),
			zap.String("rewardAddress", candidate.RewardAddress),
			zap.String("score", candidate.Votes.String()),
		)
	}
	return sm.PutState(candidatesutil.ConstructKey(height), &candidates)
}
