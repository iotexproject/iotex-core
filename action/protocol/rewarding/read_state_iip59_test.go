// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestReadStateIIP59(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	candID := identityset.Address(4)
	voter := identityset.Address(8)
	r.NoError(staking.TestOnlyPutCandidateRewardAddress(
		sm, candID, candID, identityset.Address(6), true, false,
	))

	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID.Bytes(), big.NewInt(351)))
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		epochDrainPlan: epochDrainPlan{
			TargetEra:      42,
			FreezeHeight:   iip59FixtureFreezeHeight,
			SettlementSeed: []byte{1, 2, 3},
			Delegates: []epochDrainDelegateWork{{
				CandidateIdentifier: candID.Bytes(),
				VoterAmountFrozen:   big.NewInt(300),
				SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
			}},
		},
		epochDrainProgress: epochDrainProgress{
			ScanPhase:       voterScanDone,
			CompletedHeight: 12345,
		},
	}))
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candID, &staking.CandidatePollSnapshot{
		OnchainRewardEnabled:       true,
		BlockCommissionBasisPoints: 2000,
		EpochCommissionBasisPoints: 3000,
		Registered:                 true,
		TotalWeight:                big.NewInt(99),
		FreezeHeight:               iip59FixtureFreezeHeight,
		SelfStakeBucketIdx:         staking.NoSelfStakeBucketIndex,
	}))

	pool, _, err := p.ReadState(ctx, sm, []byte("PendingBlockRewardPool"), []byte(candID.String()))
	r.NoError(err)
	r.Equal("351", string(pool))

	indexData, _, err := p.ReadState(ctx, sm, []byte("PendingBlockRewardPoolIndex"))
	r.NoError(err)
	index := &rewardingpb.PendingBlockRewardPoolIndex{}
	r.NoError(proto.Unmarshal(indexData, index))
	r.Equal([][]byte{candID.Bytes()}, index.GetCandidateIdentifiers())

	cursorData, _, err := p.ReadState(ctx, sm, []byte("EpochDrainCursor"))
	r.NoError(err)
	cursor := &rewardingpb.EpochDrainCursor{}
	r.NoError(proto.Unmarshal(cursorData, cursor))
	r.Equal(uint64(42), cursor.GetTargetEra())
	r.True(cursor.GetCompleted())
	r.Equal(uint64(12345), cursor.GetCompletedHeight())
	r.Equal([]byte{1, 2, 3}, cursor.GetSettlementSeed())
	r.Equal(settlementStartVoter([]byte{1, 2, 3}), cursor.GetStartVoter())
	r.Equal(uint32(voterScanDone), cursor.GetScanPhase())
	r.Empty(cursor.GetResumeVoter())
	r.Len(cursor.GetDelegates(), 1)
	r.Equal(big.NewInt(300).Bytes(), cursor.GetDelegates()[0].GetVoterAmountFrozen())
	r.Equal(iip59FixtureFreezeHeight, cursor.GetFreezeHeight())

	snapshotData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardSnapshot"), []byte(candID.String()))
	r.NoError(err)
	snapshot := &stakingpb.CandidatePollSnapshot{}
	r.NoError(proto.Unmarshal(snapshotData, snapshot))
	r.Equal(uint64(2000), snapshot.GetBlockCommissionBasisPoints())
	r.Equal(uint64(3000), snapshot.GetEpochCommissionBasisPoints())
	r.True(snapshot.GetRegistered())
	r.True(snapshot.GetOnchainRewardEnabled())
	r.Equal(big.NewInt(99).Bytes(), snapshot.GetTotalWeight())
	r.Equal(iip59FixtureFreezeHeight, snapshot.GetFreezeHeight())
	r.Equal(staking.NoSelfStakeBucketIndex, snapshot.GetSelfStakeBucketIdx())

	rewardAddressData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardAddress"), []byte(candID.String()))
	r.NoError(err)
	rewardAddress := &rewardingpb.VoterRewardAddress{}
	r.NoError(proto.Unmarshal(rewardAddressData, rewardAddress))
	r.Equal(candID.Bytes(), rewardAddress.GetAddress())
	r.True(rewardAddress.GetExplicitlySet())

	recipient := identityset.Address(12)
	r.NoError(p.putState(ctx, sm, voterRewardDestinationKey(voter), &voterRewardDestination{
		recipient: recipient, updatedHeight: 99,
	}))
	destinationData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardDestination"), []byte(voter.String()))
	r.NoError(err)
	destination := &rewardingpb.VoterRewardDestination{}
	r.NoError(proto.Unmarshal(destinationData, destination))
	r.Equal(recipient.Bytes(), destination.GetRecipient())
	r.True(destination.GetExplicitlySet())
	r.Equal(uint64(99), destination.GetUpdatedHeight())
}

func TestReadStateIIP59MissingAndArguments(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	candID := identityset.Address(4)

	pool, _, err := p.ReadState(ctx, sm, []byte("PendingBlockRewardPool"), []byte(candID.String()))
	r.NoError(err)
	r.Equal("0", string(pool))

	cursorData, _, err := p.ReadState(ctx, sm, []byte("EpochDrainCursor"))
	r.NoError(err)
	cursor := &rewardingpb.EpochDrainCursor{}
	r.NoError(proto.Unmarshal(cursorData, cursor))
	r.Zero(cursor.GetTargetEra())
	r.Empty(cursor.GetDelegates())

	_, _, err = p.ReadState(ctx, sm, []byte("PendingBlockRewardPool"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("PendingBlockRewardPoolIndex"), []byte("unexpected"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("EpochDrainCursor"), []byte("unexpected"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("VoterRewardSnapshot"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("VoterRewardAddress"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("VoterRewardDestination"))
	r.Error(err)
	// VoterRewardStatus is per-voter now, so it takes exactly one argument;
	// both the old two-argument form and the empty form are rejected.
	_, _, err = p.ReadState(ctx, sm, []byte("VoterRewardStatus"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("VoterRewardStatus"),
		[]byte(candID.String()), []byte(identityset.Address(8).String()))
	r.Error(err)

	voter := identityset.Address(8)
	destinationData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardDestination"), []byte(voter.String()))
	r.NoError(err)
	destination := &rewardingpb.VoterRewardDestination{}
	r.NoError(proto.Unmarshal(destinationData, destination))
	r.Equal(voter.Bytes(), destination.GetRecipient())
	r.False(destination.GetExplicitlySet())
	r.Zero(destination.GetUpdatedHeight())
}

// TestReadStateVoterRewardStatus pins the post-P5 shape of the status query:
// one argument (the voter), one amount covering every delegate that voter is
// owed by, and an amount produced by the very function the drain pays from.
func TestReadStateVoterRewardStatus(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	delegateA, delegateB := identityset.Address(4), identityset.Address(5)
	voterA, voterB := identityset.Address(8), identityset.Address(9)

	readStatus := func(voter address.Address) *rewardingpb.VoterRewardStatus {
		data, _, err := p.ReadState(ctx, sm, []byte("VoterRewardStatus"), []byte(voter.String()))
		r.NoError(err)
		status := &rewardingpb.VoterRewardStatus{}
		r.NoError(proto.Unmarshal(data, status))
		return status
	}

	r.Equal(rewardingpb.VoterRewardStatus_NO_ACTIVE_SETTLEMENT, readStatus(voterA).GetStatus(),
		"no cursor means no settlement to report on")

	const rau = int64(1_000_000_000_000_000_000)
	f := seedIIP59DrainState(t, ctx, sm, iip59FixtureFreezeHeight, []iip59NativeSeed{
		{delegate: delegateA, voter: voterA, amount: rau},
		{delegate: delegateA, voter: voterB, amount: 3 * rau},
		{delegate: delegateB, voter: voterA, amount: 5 * rau},
	}, nil)

	poolA, poolB := big.NewInt(101), big.NewInt(77)
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		epochDrainPlan: epochDrainPlan{
			TargetEra:    42,
			FreezeHeight: iip59FixtureFreezeHeight,
			Delegates: []epochDrainDelegateWork{
				newStatusWork(delegateA, poolA, f.totalWeightOf(delegateA)),
				newStatusWork(delegateB, poolB, f.totalWeightOf(delegateB)),
			},
		},
	}))

	statusA := readStatus(voterA)
	r.Equal(uint64(42), statusA.GetTargetEra())
	r.Equal(uint64(19), statusA.GetEraStartEpoch())
	r.Equal(uint64(42), statusA.GetEraEndEpoch())
	// One number covering both delegates -- this is the shape change.
	wantA := new(big.Int).Add(
		f.expectedShare(delegateA, voterA, poolA),
		f.expectedShare(delegateB, voterA, poolB),
	)
	r.Zero(new(big.Int).SetBytes(statusA.GetRewardAmount()).Cmp(wantA))
	r.True(wantA.Sign() > 0, "fixture must give voterA a non-zero claim")

	statusB := readStatus(voterB)
	wantB := f.expectedShare(delegateA, voterB, poolA)
	r.Zero(new(big.Int).SetBytes(statusB.GetRewardAmount()).Cmp(wantB))

	// Neither delegate is reported as owing more than it froze.
	r.True(new(big.Int).Add(
		f.expectedShare(delegateA, voterA, poolA),
		f.expectedShare(delegateA, voterB, poolA),
	).Cmp(poolA) <= 0)

	r.Equal(rewardingpb.VoterRewardStatus_VOTER_NOT_INCLUDED,
		readStatus(identityset.Address(11)).GetStatus(),
		"an address with no frozen bucket is not part of the settlement")
}

// TestReadStateVoterRewardStatusCircularPosition checks PROCESSED/WAITING
// against the seed-derived start address, the single wrap, and the exclusive
// resume point. The exact start belongs to the tail range.
func TestReadStateVoterRewardStatusCircularPosition(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	delegate := identityset.Address(4)
	voter := identityset.Address(8)

	const rau = int64(1_000_000_000_000_000_000)
	f := seedIIP59DrainState(t, ctx, sm, iip59FixtureFreezeHeight, []iip59NativeSeed{
		{delegate: delegate, voter: voter, amount: rau},
	}, nil)

	work := newStatusWork(delegate, big.NewInt(1000), f.totalWeightOf(delegate))
	lowStart := make([]byte, 20)
	highStart := bytes.Repeat([]byte{0xFF}, 20)
	exactStart := append([]byte(nil), voter.Bytes()...)

	readStatus := func(start []byte, phase voterScanPhase, resume []byte) rewardingpb.VoterRewardStatus_Status {
		cursor := &epochDrainCursor{
			epochDrainPlan: epochDrainPlan{
				TargetEra: 42, FreezeHeight: iip59FixtureFreezeHeight,
				SettlementSeed: append([]byte(nil), start...),
				Delegates:      []epochDrainDelegateWork{work},
			},
			epochDrainProgress: epochDrainProgress{
				ScanPhase:   phase,
				ResumeVoter: append([]byte(nil), resume...),
			},
		}
		r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))
		data, _, err := p.ReadState(ctx, sm, []byte("VoterRewardStatus"), []byte(voter.String()))
		r.NoError(err)
		status := &rewardingpb.VoterRewardStatus{}
		r.NoError(proto.Unmarshal(data, status))
		return status.GetStatus()
	}

	// Voter lies in the tail but the cursor has not reached it yet.
	r.Equal(rewardingpb.VoterRewardStatus_WAITING,
		readStatus(lowStart, voterScanTail, nil))
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED,
		readStatus(lowStart, voterScanTail, voter.Bytes()))

	// Voter lies in the head: it remains waiting before and after the wrap
	// until the head resume point reaches it.
	r.Equal(rewardingpb.VoterRewardStatus_WAITING,
		readStatus(highStart, voterScanTail, nil))
	r.Equal(rewardingpb.VoterRewardStatus_WAITING,
		readStatus(highStart, voterScanHead, nil))
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED,
		readStatus(highStart, voterScanHead, voter.Bytes()))

	// Once the walk wraps, every address in the completed tail is processed.
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED,
		readStatus(lowStart, voterScanHead, nil))

	// Equality belongs to [start,max], not the wrapped [min,start) range.
	r.Equal(rewardingpb.VoterRewardStatus_WAITING,
		readStatus(exactStart, voterScanTail, nil))
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED,
		readStatus(exactStart, voterScanTail, voter.Bytes()))
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED,
		readStatus(exactStart, voterScanHead, nil))

	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED,
		readStatus(highStart, voterScanDone, nil))
}

// newStatusWork builds a payable frozen work item for the status tests.
func newStatusWork(delegate address.Address, pool, totalWeight *big.Int) epochDrainDelegateWork {
	return epochDrainDelegateWork{
		CandidateIdentifier: delegate.Bytes(),
		VoterAmountFrozen:   pool,
		TotalWeight:         totalWeight,
		SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
	}
}
