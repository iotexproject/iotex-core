// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
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
		TargetEra:       42,
		StartEpoch:      19,
		EndEpoch:        42,
		StartShard:      5,
		ShardsDone:      7,
		ResumeVoter:     voter.Bytes(),
		SettlementSeed:  []byte{1, 2, 3},
		Completed:       true,
		CompletedHeight: 12345,
		Delegates: []epochDrainDelegateWork{{
			CandidateIdentifier: candID.Bytes(),
			VoterAmountFrozen:   big.NewInt(300),
			FreezeHeight:        iip59FixtureFreezeHeight,
			SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
		}},
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
	r.Equal(uint64(19), cursor.GetStartEpoch())
	r.Equal(uint64(42), cursor.GetEndEpoch())
	r.True(cursor.GetCompleted())
	r.Equal(uint64(12345), cursor.GetCompletedHeight())
	r.Equal(uint32(5), cursor.GetStartShard())
	r.Equal(uint32(7), cursor.GetShardsDone())
	r.Equal(voter.Bytes(), cursor.GetResumeVoter())
	r.Equal([]byte{1, 2, 3}, cursor.GetSettlementSeed())
	r.Len(cursor.GetDelegates(), 1)
	r.Equal(big.NewInt(300).Bytes(), cursor.GetDelegates()[0].GetVoterAmountFrozen())
	r.Equal(iip59FixtureFreezeHeight, cursor.GetDelegates()[0].GetFreezeHeight())

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
	r.NotEmpty(snapshot.GetSnapshotHash())

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
		TargetEra: 42, StartEpoch: 19, EndEpoch: 42,
		Delegates: []epochDrainDelegateWork{
			newStatusWork(delegateA, poolA, f.totalWeightOf(delegateA)),
			newStatusWork(delegateB, poolB, f.totalWeightOf(delegateB)),
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

// TestReadStateVoterRewardStatusShardPosition checks PROCESSED/WAITING is read
// off the shard rotation rather than off a delegate index that no longer
// exists. Position is measured from StartShard, so the answer must depend on
// the rotation and not on the raw shard id.
func TestReadStateVoterRewardStatusShardPosition(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	delegate := identityset.Address(4)
	voter := identityset.Address(8)

	const rau = int64(1_000_000_000_000_000_000)
	f := seedIIP59DrainState(t, ctx, sm, iip59FixtureFreezeHeight, []iip59NativeSeed{
		{delegate: delegate, voter: voter, amount: rau},
	}, nil)

	work := newStatusWork(delegate, big.NewInt(1000), f.totalWeightOf(delegate))
	shard := staking.ShardOf(voter)

	readStatus := func(cursor *epochDrainCursor) rewardingpb.VoterRewardStatus_Status {
		r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))
		data, _, err := p.ReadState(ctx, sm, []byte("VoterRewardStatus"), []byte(voter.String()))
		r.NoError(err)
		status := &rewardingpb.VoterRewardStatus{}
		r.NoError(proto.Unmarshal(data, status))
		return status.GetStatus()
	}

	// The walk starts at the voter's own shard and has not entered it yet.
	r.Equal(rewardingpb.VoterRewardStatus_WAITING, readStatus(&epochDrainCursor{
		TargetEra: 42, StartShard: shard, ShardsDone: 0,
		Delegates: []epochDrainDelegateWork{work},
	}))
	// Inside that shard, past this voter.
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED, readStatus(&epochDrainCursor{
		TargetEra: 42, StartShard: shard, ShardsDone: 0, ResumeVoter: voter.Bytes(),
		Delegates: []epochDrainDelegateWork{work},
	}))
	// That shard is finished.
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED, readStatus(&epochDrainCursor{
		TargetEra: 42, StartShard: shard, ShardsDone: 1,
		Delegates: []epochDrainDelegateWork{work},
	}))
	// Same raw shard id, but the rotation starts one past it, so the voter's
	// shard is now the last one visited rather than the first.
	r.Equal(rewardingpb.VoterRewardStatus_WAITING, readStatus(&epochDrainCursor{
		TargetEra: 42, StartShard: shard + 1, ShardsDone: 1,
		Delegates: []epochDrainDelegateWork{work},
	}))
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED, readStatus(&epochDrainCursor{
		TargetEra: 42, Completed: true, CompletedHeight: 900,
		Delegates: []epochDrainDelegateWork{work},
	}))
}

// newStatusWork builds a payable frozen work item for the status tests.
func newStatusWork(delegate address.Address, pool, totalWeight *big.Int) epochDrainDelegateWork {
	return epochDrainDelegateWork{
		CandidateIdentifier: delegate.Bytes(),
		VoterAmountFrozen:   pool,
		TotalWeight:         totalWeight,
		FreezeHeight:        iip59FixtureFreezeHeight,
		SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
	}
}
