// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
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
		ctx, sm, candID, candID, identityset.Address(6), true, false,
	))

	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID.Bytes(), big.NewInt(351)))
	r.NoError(p.writeVoterRewardDistributionState(ctx, sm, &voterRewardDistributionState{
		voterRewardDistributionPlan: voterRewardDistributionPlan{
			TargetEra:      42,
			FreezeHeight:   iip59FixtureFreezeHeight,
			SettlementSeed: []byte{1, 2, 3},
			DelegateAllocations: []voterRewardDelegateAllocation{{
				CandidateIdentifier: candID.Bytes(),
				VoterAmountFrozen:   big.NewInt(300),
				SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
			}},
		},
		voterRewardDistributionProgress: voterRewardDistributionProgress{
			ScanPhase:       voterScanDone,
			CompletedHeight: 12345,
		},
	}))
	r.NoError(staking.TestOnlyPutCandidateRewardSnapshotFor(sm, candID, &staking.CandidateRewardSnapshot{
		BlockCommissionBasisPoints: 2000,
		EpochCommissionBasisPoints: 3000,
		CommissionConfigured:       true,
		TotalWeight:                big.NewInt(99),
		FreezeHeight:               iip59FixtureFreezeHeight,
		SelfStakeBucketIdx:         staking.NoSelfStakeBucketIndex,
	}))

	pool, _, err := p.ReadState(ctx, sm, []byte("PendingVoterReward"), []byte(candID.String()))
	r.NoError(err)
	r.Equal("351", string(pool))

	delegatesData, _, err := p.ReadState(ctx, sm, []byte("PendingVoterRewardDelegates"))
	r.NoError(err)
	delegates := &rewardingpb.PendingVoterRewardDelegates{}
	r.NoError(proto.Unmarshal(delegatesData, delegates))
	r.Equal([][]byte{candID.Bytes()}, delegates.GetDelegateIdentifiers())

	cursorData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardDistribution"))
	r.NoError(err)
	cursor := &rewardingpb.VoterRewardDistributionState{}
	r.NoError(proto.Unmarshal(cursorData, cursor))
	r.Equal(uint64(42), cursor.GetTargetEra())
	r.True(cursor.GetCompleted())
	r.Equal(uint64(12345), cursor.GetCompletedHeight())
	r.Equal([]byte{1, 2, 3}, cursor.GetSettlementSeed())
	r.Equal(settlementStartVoter([]byte{1, 2, 3}), cursor.GetStartVoter())
	r.Equal(uint32(voterScanDone), cursor.GetScanPhase())
	r.Empty(cursor.GetResumeVoter())
	r.Len(cursor.GetDelegateAllocations(), 1)
	r.Equal(big.NewInt(300).Bytes(), cursor.GetDelegateAllocations()[0].GetVoterAmountFrozen())
	r.Equal(iip59FixtureFreezeHeight, cursor.GetFreezeHeight())

	snapshotData, _, err := p.ReadState(ctx, sm, []byte("DelegateRewardSnapshot"), []byte(candID.String()))
	r.NoError(err)
	snapshot := &stakingpb.CandidateRewardSnapshot{}
	r.NoError(proto.Unmarshal(snapshotData, snapshot))
	r.Equal(uint64(2000), snapshot.GetBlockCommissionBasisPoints())
	r.Equal(uint64(3000), snapshot.GetEpochCommissionBasisPoints())
	r.True(snapshot.GetCommissionConfigured())
	r.Equal(big.NewInt(99).Bytes(), snapshot.GetTotalWeight())
	r.Equal(iip59FixtureFreezeHeight, snapshot.GetFreezeHeight())
	r.Equal(staking.NoSelfStakeBucketIndex, snapshot.GetSelfStakeBucketIdx())

	rewardAddressData, _, err := p.ReadState(ctx, sm, []byte("DelegatePayoutAddress"), []byte(candID.String()))
	r.NoError(err)
	rewardAddress := &rewardingpb.DelegatePayoutAddress{}
	r.NoError(proto.Unmarshal(rewardAddressData, rewardAddress))
	r.Equal(candID.Bytes(), rewardAddress.GetAddress())
	r.True(rewardAddress.GetOnchainRewardEnabled())

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

	pool, _, err := p.ReadState(ctx, sm, []byte("PendingVoterReward"), []byte(candID.String()))
	r.NoError(err)
	r.Equal("0", string(pool))

	cursorData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardDistribution"))
	r.NoError(err)
	cursor := &rewardingpb.VoterRewardDistributionState{}
	r.NoError(proto.Unmarshal(cursorData, cursor))
	r.Zero(cursor.GetTargetEra())
	r.Empty(cursor.GetDelegateAllocations())

	_, _, err = p.ReadState(ctx, sm, []byte("PendingVoterReward"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("PendingVoterRewardDelegates"), []byte("unexpected"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("VoterRewardDistribution"), []byte("unexpected"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("DelegateRewardSnapshot"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("DelegatePayoutAddress"))
	r.Error(err)
	_, _, err = p.ReadState(ctx, sm, []byte("VoterRewardDestination"))
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

// TestReadStateDelegateRewardSnapshotAbsent pins the answer for a delegate that
// is not in the settlement set.
//
// That is what every delegate which has not opted in looks like -- an ordinary
// state, not a fault. It used to revert with "state does not exist", so a caller
// needed a try/catch to ask an ordinary question.
func TestReadStateDelegateRewardSnapshotAbsent(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	absent := identityset.Address(9)

	data, _, err := p.ReadState(ctx, sm, []byte("DelegateRewardSnapshot"), []byte(absent.String()))
	r.NoError(err, "a delegate outside the settlement set is a normal answer")

	snapshot := &stakingpb.CandidateRewardSnapshot{}
	r.NoError(proto.Unmarshal(data, snapshot))

	// freezeHeight is the discriminator: a real snapshot always carries the
	// height its freeze ran at, and a freeze cannot run at height 0.
	r.Zero(snapshot.GetFreezeHeight(), "freezeHeight == 0 is how a caller detects absence")
	r.Zero(snapshot.GetBlockCommissionBasisPoints())
	r.Zero(snapshot.GetEpochCommissionBasisPoints())
	r.False(snapshot.GetCommissionConfigured())
	r.Empty(snapshot.GetTotalWeight())
	r.Zero(snapshot.GetSelfStakeBucketIdx())

	// A malformed identifier is still an error -- only absence was reclassified.
	_, _, err = p.ReadState(ctx, sm, []byte("DelegateRewardSnapshot"), []byte("not-an-address"))
	r.Error(err)
}
