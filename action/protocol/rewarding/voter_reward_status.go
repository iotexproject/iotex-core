// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// voterRewardStatus reports what one voter is owed by the active or most
// recently completed settlement, and whether the drain has reached them yet.
//
// It is per-voter, not per (candidate, voter): the drain pays a voter once for
// everything they are owed across every delegate, so a per-candidate answer
// would no longer correspond to anything that moves on chain.
//
// The amount comes from computeVoterShares -- the same function, with the same
// payout clamp, that the drain itself uses. That is deliberate: it is what
// makes "the number this query reports is the number the drain pays" a property
// of the code rather than of two implementations happening to agree.
func (p *Protocol) voterRewardStatus(
	ctx context.Context,
	sr protocol.StateReader,
	voter address.Address,
) (*rewardingpb.VoterRewardStatus, error) {
	status := &rewardingpb.VoterRewardStatus{
		Status:       rewardingpb.VoterRewardStatus_NO_ACTIVE_SETTLEMENT,
		RewardAmount: new(big.Int).Bytes(),
	}
	cursor, err := p.readEpochDrainCursor(ctx, sr)
	if err != nil {
		return nil, err
	}
	if cursor == nil {
		return status, nil
	}
	status.TargetEra = cursor.TargetEra
	g := genesis.MustExtractGenesisContext(ctx)
	status.EraStartEpoch = rewardEraStartEpoch(cursor.TargetEra, g.EpochsPerRewardEra)
	status.EraEndEpoch = cursor.TargetEra
	status.SettlementCompleted = cursor.drainFinished()
	status.CompletedHeight = cursor.CompletedHeight

	window, err := staking.EraCOWWindow(sr)
	if err != nil {
		return nil, err
	}
	if !window.Open() {
		// The era's frozen bucket copies are what the amount is computed from,
		// and they are sealed the moment the drain finishes. There is no honest
		// number to report once they are gone, so say so rather than recompute
		// against live state and hand back a figure nobody was ever paid.
		status.Status = rewardingpb.VoterRewardStatus_SNAPSHOT_UNAVAILABLE
		return status, nil
	}
	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProto == nil {
		return nil, errors.New("rewarding: staking protocol not registered")
	}

	shares, err := computeVoterShares(sr, voterShareInputs{
		window:       window,
		staking:      stakingProto,
		delegates:    cursor.Delegates,
		byCandidate:  delegateWorkIndex(cursor.Delegates),
		freezeHeight: cursor.FreezeHeight,
		distributed:  distributedVector(cursor),
	}, voter)
	if err != nil {
		return nil, err
	}
	if len(shares.shares) == 0 {
		status.Status = rewardingpb.VoterRewardStatus_VOTER_NOT_INCLUDED
		return status, nil
	}
	status.RewardAmount = shares.total.Bytes()
	status.Status = voterDrainPosition(cursor, voter)
	return status, nil
}

// voterDrainPosition reports whether the circular address walk has passed a
// voter. The tail phase covers [start, max]; after the single wrap, the head
// phase covers [min, start). ResumeVoter is the exclusive lower bound already
// covered inside the current phase.
func voterDrainPosition(cursor *epochDrainCursor, voter address.Address) rewardingpb.VoterRewardStatus_Status {
	if cursor.drainFinished() {
		return rewardingpb.VoterRewardStatus_PROCESSED
	}
	addr := voter.Bytes()
	start := settlementStartVoter(cursor.SettlementSeed)
	switch cursor.ScanPhase {
	case voterScanTail:
		if bytes.Compare(addr, start) < 0 {
			return rewardingpb.VoterRewardStatus_WAITING
		}
	case voterScanHead:
		if bytes.Compare(addr, start) >= 0 {
			return rewardingpb.VoterRewardStatus_PROCESSED
		}
	default:
		return rewardingpb.VoterRewardStatus_WAITING
	}
	if len(cursor.ResumeVoter) > 0 && bytes.Compare(addr, cursor.ResumeVoter) <= 0 {
		return rewardingpb.VoterRewardStatus_PROCESSED
	}
	return rewardingpb.VoterRewardStatus_WAITING
}

// distributedVector returns an isolated, nil-free view for the read-only
// reward query. The drain itself aliases cursor.Distributed so each payout is
// visible to the next voter in the same block.
func distributedVector(c *epochDrainCursor) []*big.Int {
	out := make([]*big.Int, len(c.Delegates))
	for i := range out {
		out[i] = c.distributedAt(i)
	}
	return out
}
