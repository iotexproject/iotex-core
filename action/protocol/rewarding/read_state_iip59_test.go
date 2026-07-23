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
	csm, err := staking.NewCandidateStateManager(sm)
	r.NoError(err)
	r.NoError(csm.Upsert(&staking.Candidate{
		Owner: candID, Operator: identityset.Address(5), Reward: identityset.Address(6),
		Name: "iip59-read-state", Votes: big.NewInt(1), SelfStake: big.NewInt(0),
		RewardAddressUpdated: true,
	}))
	r.NoError(csm.Commit(ctx))

	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID.Bytes(), big.NewInt(351)))
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra:     42,
		DelegateIndex: 1,
		VoterIndex:    7,
		Delegates: []epochDrainDelegateWork{{
			CandidateIdentifier: candID.Bytes(),
			VoterAmountFrozen:   big.NewInt(300),
		}},
	}))
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candID, &staking.CandidatePollSnapshot{
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
	index := &rewardingpb.Exempt{}
	r.NoError(proto.Unmarshal(indexData, index))
	r.Equal([][]byte{candID.Bytes()}, index.GetAddrs())

	cursorData, _, err := p.ReadState(ctx, sm, []byte("EpochDrainCursor"))
	r.NoError(err)
	cursor := &rewardingpb.EpochDrainCursor{}
	r.NoError(proto.Unmarshal(cursorData, cursor))
	r.Equal(uint64(42), cursor.GetTargetEra())
	r.Equal(uint32(1), cursor.GetDelegateIndex())
	r.Equal(uint32(7), cursor.GetVoterIndex())
	r.Len(cursor.GetDelegates(), 1)
	r.Equal(big.NewInt(300).Bytes(), cursor.GetDelegates()[0].GetVoterAmountFrozen())

	snapshotData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardSnapshot"), []byte(candID.String()))
	r.NoError(err)
	snapshot := &stakingpb.CandidatePollSnapshot{}
	r.NoError(proto.Unmarshal(snapshotData, snapshot))
	r.Equal(uint64(2000), snapshot.GetBlockCommissionBasisPoints())
	r.Equal(uint64(3000), snapshot.GetEpochCommissionBasisPoints())
	r.True(snapshot.GetRegistered())
	r.Len(snapshot.GetEntries(), 1)
	r.Equal(voter.Bytes(), snapshot.GetEntries()[0].GetVoter())
	r.Equal(big.NewInt(99).Bytes(), snapshot.GetEntries()[0].GetWeight())

	rewardAddressData, _, err := p.ReadState(ctx, sm, []byte("VoterRewardAddress"), []byte(candID.String()))
	r.NoError(err)
	rewardAddress := &rewardingpb.VoterRewardAddress{}
	r.NoError(proto.Unmarshal(rewardAddressData, rewardAddress))
	r.Equal(identityset.Address(6).Bytes(), rewardAddress.GetAddress())
	r.True(rewardAddress.GetExplicitlySet())
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
}
