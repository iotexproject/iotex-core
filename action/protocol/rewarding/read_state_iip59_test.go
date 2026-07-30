// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"math/big"
	"sort"
	"testing"

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
		sm, candID, candID, identityset.Address(6), true,
	))

	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID.Bytes(), big.NewInt(351)))
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra:          42,
		StartEpoch:         19,
		EndEpoch:           42,
		DelegateIndex:      1,
		VoterIndex:         7,
		SettlementSeed:     []byte{1, 2, 3},
		DelegateStartIndex: 5,
		Completed:          true,
		CompletedHeight:    12345,
		Delegates: []epochDrainDelegateWork{{
			CandidateIdentifier: candID.Bytes(),
			VoterAmountFrozen:   big.NewInt(300),
			VoterStartIndex:     11,
		}},
	}))
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candID, &staking.CandidatePollSnapshot{
		OnchainRewardEnabled:       true,
		BlockCommissionBasisPoints: 2000,
		EpochCommissionBasisPoints: 3000,
		Registered:                 true,
		Entries:                    []staking.VoterWeight{{Voter: voter, Weight: big.NewInt(99)}},
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
	r.Equal(uint32(1), cursor.GetDelegateIndex())
	r.Equal(uint32(7), cursor.GetVoterIndex())
	r.Equal([]byte{1, 2, 3}, cursor.GetSettlementSeed())
	r.Equal(uint32(5), cursor.GetDelegateStartIndex())
	r.Len(cursor.GetDelegates(), 1)
	r.Equal(big.NewInt(300).Bytes(), cursor.GetDelegates()[0].GetVoterAmountFrozen())
	r.Equal(uint32(11), cursor.GetDelegates()[0].GetVoterStartIndex())

	snapshotData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardSnapshot"), []byte(candID.String()))
	r.NoError(err)
	snapshot := &stakingpb.CandidatePollSnapshot{}
	r.NoError(proto.Unmarshal(snapshotData, snapshot))
	r.Equal(uint64(2000), snapshot.GetBlockCommissionBasisPoints())
	r.Equal(uint64(3000), snapshot.GetEpochCommissionBasisPoints())
	r.True(snapshot.GetRegistered())
	r.True(snapshot.GetOnchainRewardEnabled())
	r.Len(snapshot.GetEntries(), 1)
	r.Equal(voter.Bytes(), snapshot.GetEntries()[0].GetVoter())
	r.Equal(big.NewInt(99).Bytes(), snapshot.GetEntries()[0].GetWeight())

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
	_, _, err = p.ReadState(ctx, sm, []byte("VoterRewardStatus"), []byte(candID.String()))
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

func TestReadStateVoterRewardStatus(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	candID := identityset.Address(4)
	voters := []staking.VoterWeight{
		{Voter: identityset.Address(8), Weight: big.NewInt(1)},
		{Voter: identityset.Address(9), Weight: big.NewInt(2)},
		{Voter: identityset.Address(10), Weight: big.NewInt(3)},
	}
	sort.Slice(voters, func(i, j int) bool {
		return bytes.Compare(voters[i].Voter.Bytes(), voters[j].Voter.Bytes()) < 0
	})
	for i := range voters {
		voters[i].Weight = big.NewInt(int64(i + 1))
	}

	readStatus := func(candidate, voter string) *rewardingpb.VoterRewardStatus {
		data, _, err := p.ReadState(
			ctx, sm, []byte("VoterRewardStatus"), []byte(candidate), []byte(voter),
		)
		r.NoError(err)
		status := &rewardingpb.VoterRewardStatus{}
		r.NoError(proto.Unmarshal(data, status))
		return status
	}

	status := readStatus(candID.String(), voters[0].Voter.String())
	r.Equal(rewardingpb.VoterRewardStatus_NO_ACTIVE_SETTLEMENT, status.GetStatus())
	r.Zero(status.GetTargetEra())

	snapshot := &staking.CandidatePollSnapshot{Entries: voters}
	const voterStartIndex = uint32(1)
	totalWeight, snapshotHash, lastWeightedIndex, hasWeightedEntries := voterDistributionMetadata(
		snapshot, voterStartIndex,
	)
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candID, snapshot))
	work := epochDrainDelegateWork{
		CandidateIdentifier: candID.Bytes(),
		VoterAmountFrozen:   big.NewInt(101),
		TotalWeight:         totalWeight,
		SnapshotHash:        snapshotHash[:],
		LastWeightedIndex:   lastWeightedIndex,
		HasWeightedEntries:  hasWeightedEntries,
		VoterStartIndex:     voterStartIndex,
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra:     42,
		DelegateIndex: 0,
		VoterIndex:    1,
		Delegates:     []epochDrainDelegateWork{work},
	}))

	status = readStatus(candID.String(), voters[1].Voter.String())
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED, status.GetStatus())
	r.Equal(uint64(42), status.GetTargetEra())
	r.Equal(uint32(0), status.GetLogicalVoterIndex())
	r.Equal(voterStartIndex, status.GetVoterStartIndex())
	r.Equal(big.NewInt(33), new(big.Int).SetBytes(status.GetRewardAmount()))

	status = readStatus(candID.String(), voters[2].Voter.String())
	r.Equal(rewardingpb.VoterRewardStatus_WAITING, status.GetStatus())
	r.Equal(uint32(1), status.GetLogicalVoterIndex())
	r.Equal(big.NewInt(50), new(big.Int).SetBytes(status.GetRewardAmount()))

	// The final positive voter gets the two-rau integer-division remainder.
	status = readStatus(candID.String(), voters[0].Voter.String())
	r.Equal(rewardingpb.VoterRewardStatus_WAITING, status.GetStatus())
	r.Equal(uint32(2), status.GetLogicalVoterIndex())
	r.Equal(big.NewInt(18), new(big.Int).SetBytes(status.GetRewardAmount()))

	status = readStatus(identityset.Address(5).String(), voters[0].Voter.String())
	r.Equal(rewardingpb.VoterRewardStatus_CANDIDATE_NOT_INCLUDED, status.GetStatus())
	status = readStatus(candID.String(), identityset.Address(11).String())
	r.Equal(rewardingpb.VoterRewardStatus_VOTER_NOT_INCLUDED, status.GetStatus())

	work.VoterAmountDistributed = big.NewInt(101)
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra: 42, StartEpoch: 19, EndEpoch: 42,
		DelegateIndex: 1, Completed: true, CompletedHeight: 12345,
		Delegates: []epochDrainDelegateWork{work},
	}))
	status = readStatus(candID.String(), voters[0].Voter.String())
	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED, status.GetStatus())
	r.Equal(uint64(19), status.GetEraStartEpoch())
	r.Equal(uint64(42), status.GetEraEndEpoch())
	r.True(status.GetSettlementCompleted())
	r.Equal(uint64(12345), status.GetCompletedHeight())
	r.Equal(big.NewInt(18), new(big.Int).SetBytes(status.GetRewardAmount()))

	work.SnapshotHash = []byte{1}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra: 42, Delegates: []epochDrainDelegateWork{work},
	}))
	status = readStatus(candID.String(), voters[0].Voter.String())
	r.Equal(rewardingpb.VoterRewardStatus_SNAPSHOT_UNAVAILABLE, status.GetStatus())
}

func TestReadStateVoterRewardStatusDelegateProgress(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	candID := identityset.Address(4)
	voter := identityset.Address(8)
	snapshot := &staking.CandidatePollSnapshot{Entries: []staking.VoterWeight{{
		Voter: voter, Weight: big.NewInt(1),
	}}}
	totalWeight, snapshotHash, lastWeightedIndex, hasWeightedEntries := voterDistributionMetadata(snapshot, 0)
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candID, snapshot))
	work := epochDrainDelegateWork{
		CandidateIdentifier: candID.Bytes(), VoterAmountFrozen: big.NewInt(7),
		TotalWeight: totalWeight, SnapshotHash: snapshotHash[:],
		LastWeightedIndex: lastWeightedIndex, HasWeightedEntries: hasWeightedEntries,
	}
	other := work
	other.CandidateIdentifier = identityset.Address(5).Bytes()

	readStatus := func(cursor *epochDrainCursor) rewardingpb.VoterRewardStatus_Status {
		r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))
		data, _, err := p.ReadState(ctx, sm, []byte("VoterRewardStatus"),
			[]byte(candID.String()), []byte(voter.String()))
		r.NoError(err)
		status := &rewardingpb.VoterRewardStatus{}
		r.NoError(proto.Unmarshal(data, status))
		return status.GetStatus()
	}

	r.Equal(rewardingpb.VoterRewardStatus_PROCESSED, readStatus(&epochDrainCursor{
		TargetEra: 42, DelegateIndex: 1, Delegates: []epochDrainDelegateWork{work, other},
	}))
	r.Equal(rewardingpb.VoterRewardStatus_WAITING, readStatus(&epochDrainCursor{
		TargetEra: 42, DelegateIndex: 0, Delegates: []epochDrainDelegateWork{other, work},
	}))
}
