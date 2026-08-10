// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestActivationBackfillCOWDrainPayout is the bridge between the activation,
// era-COW, and rewarding tests. The component tests on either side cannot
// catch a layout or lifecycle mismatch between them: activation tests stop at
// FrozenContractBucket, while drain tests seed owner refs and high-water marks
// directly instead of obtaining them from CreatePreStates.
func TestActivationBackfillCOWDrainPayout(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, delegate := newVoterRewardCtx(t, true)
	g := genesis.MustExtractGenesisContext(ctx)
	sp := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	r.NotNil(sp)

	contractAt := func(raw string) address.Address {
		addr, err := address.FromString(raw)
		r.NoError(err)
		return addr
	}
	v1 := contractAt(g.SystemStakingContractAddress)
	v2 := contractAt(g.SystemStakingContractV2Address)
	v3 := contractAt(g.SystemStakingContractV3Address)
	alice := identityset.Address(8)
	bob := identityset.Address(9)
	carol := identityset.Address(10)
	dave := identityset.Address(11)
	eve := identityset.Address(12)

	activationCtx := iip59BackfillDrainCtx(ctx, g.ToBeEnabledBlockHeight)
	createdAt := time.Unix(1, 0).UTC()
	contractBucket := func(owner address.Address, amount int64, days uint64) *contractstaking.Bucket {
		return &contractstaking.Bucket{
			Candidate:        delegate,
			Owner:            owner,
			StakedAmount:     big.NewInt(amount),
			StakedDuration:   days * uint64((24*time.Hour)/time.Second),
			CreatedAt:        uint64(createdAt.Unix()),
			UnlockedAt:       staking.MaxDurationNumber,
			UnstakedAt:       staking.MaxDurationNumber,
			IsTimestampBased: true,
		}
	}
	oracleContractWeight := func(b *contractstaking.Bucket) *big.Int {
		vb := staking.NewVoteBucket(
			b.Candidate,
			b.Owner,
			new(big.Int).Set(b.StakedAmount),
			uint32(b.StakedDuration/uint64((24*time.Hour)/time.Second)),
			time.Unix(int64(b.CreatedAt), 0),
			true,
		)
		return staking.CalculateVoteWeight(g.VoteWeightCalConsts, vb, false)
	}

	// These four buckets predate IIP-59. A context with no feature gate writes
	// only the bucket values: no owner refs and no V2/V3 high-water marks.
	preFork := contractstaking.NewContractStakingStateManager(sm)
	v1Alice := contractBucket(alice, 100, 30)
	v2Alice := contractBucket(alice, 200, 60)
	v3Bob := contractBucket(bob, 300, 90)
	v1Carol := contractBucket(carol, 400, 120)
	r.NoError(preFork.UpsertBucket(context.Background(), v1, 1, v1Alice))
	r.NoError(preFork.UpsertBucket(context.Background(), v2, 2, v2Alice))
	r.NoError(preFork.UpsertBucket(context.Background(), v3, 3, v3Bob))
	r.NoError(preFork.UpsertBucket(context.Background(), v1, 4, v1Carol))

	// Alice is deliberately present in both enumeration streams. The drain must
	// visit her once and combine her native and contract weights.
	nativeAmount := big.NewInt(50)
	_, err := staking.TestOnlySeedNativeVoterBucket(
		sm, delegate, alice, nativeAmount, 45, createdAt, true,
	)
	r.NoError(err)
	nativeWeight := staking.CalculateVoteWeight(
		g.VoteWeightCalConsts,
		staking.NewVoteBucket(delegate, alice, nativeAmount, 45, createdAt, true),
		false,
	)

	weights := map[string]*big.Int{
		alice.String(): new(big.Int).Add(
			nativeWeight,
			new(big.Int).Add(oracleContractWeight(v1Alice), oracleContractWeight(v2Alice)),
		),
		bob.String():   oracleContractWeight(v3Bob),
		carol.String(): oracleContractWeight(v1Carol),
	}
	totalWeight := new(big.Int)
	for _, weight := range weights {
		totalWeight.Add(totalWeight, weight)
	}

	// Candidate.Votes already reflects the source buckets before the activation
	// block starts. The returned fixture candidate is not the center record the
	// freezer reads, so update through the state manager before CreatePreStates.
	csm, err := staking.NewCandidateStateManagerWithContext(activationCtx, sm)
	r.NoError(err)
	candidate := csm.GetByIdentifier(delegate)
	r.NotNil(candidate)
	candidate.Votes = new(big.Int).Set(totalWeight)
	candidate.VoterRewardOnchainOptIn = true
	r.NoError(csm.Upsert(candidate))
	r.NoError(csm.Commit(activationCtx))

	// This is the production single-block ordering: protocol pre-states build
	// the full owner index, then a PutPollResult action in that same activation
	// block can freeze the first era. There is no intervening block or context.
	r.NoError(sp.CreatePreStates(activationCtx, sm))
	r.NoError(staking.FreezePollSnapshot(activationCtx, sm, nil, nil))
	refs, _, err := contractstaking.NewStateReader(sm).BucketRefsByOwner(alice)
	r.NoError(err)
	r.Len(refs, 2, "activation must backfill both of Alice's pre-fork LSD buckets")
	snapshot, err := staking.PollSnapshotFor(sm, delegate)
	r.NoError(err)
	r.Equal(g.ToBeEnabledBlockHeight, snapshot.FreezeHeight)
	r.Zero(totalWeight.Cmp(snapshot.TotalWeight))

	// Mutate every contract generation after the freeze. The live state now
	// disagrees with the payable era in owner, existence, amount, duration, and
	// unstake status; a post-freeze bucket is also added under the old id range's
	// successor. Every payout assertion below therefore depends on COW.
	live := contractstaking.NewContractStakingStateManager(sm)
	movedAndExpanded := v1Alice.Clone()
	movedAndExpanded.Owner = dave
	movedAndExpanded.StakedAmount = big.NewInt(9_999)
	r.NoError(live.UpsertBucket(activationCtx, v1, 1, movedAndExpanded))
	r.NoError(live.DeleteBucket(activationCtx, v2, 2))
	unstaked := v3Bob.Clone()
	unstaked.UnlockedAt = uint64(createdAt.Add(91 * 24 * time.Hour).Unix())
	unstaked.UnstakedAt = unstaked.UnlockedAt + 1
	r.NoError(live.UpsertBucket(activationCtx, v3, 3, unstaked))
	restakedAndExpanded := v1Carol.Clone()
	restakedAndExpanded.StakedAmount = big.NewInt(8_888)
	restakedAndExpanded.StakedDuration = 365 * uint64((24*time.Hour)/time.Second)
	r.NoError(live.UpsertBucket(activationCtx, v1, 4, restakedAndExpanded))
	r.NoError(live.UpsertBucket(activationCtx, v3, 5, contractBucket(eve, 7_777, 365)))

	const pool = int64(1_000_003)
	r.NoError(p.putState(activationCtx, sm, _fundKey, &fund{
		totalBalance:     big.NewInt(pool * 8),
		unclaimedBalance: big.NewInt(pool * 8),
	}))
	r.NoError(p.creditPendingBlockRewardPool(activationCtx, sm, delegate.Bytes(), big.NewInt(pool)))
	r.NoError(p.updateAvailableBalance(activationCtx, sm, big.NewInt(pool)))
	cursor := &epochDrainCursor{
		epochDrainPlan: epochDrainPlan{
			TargetEra:      1,
			FreezeHeight:   snapshot.FreezeHeight,
			SettlementSeed: []byte{0x59},
			Delegates: []epochDrainDelegateWork{{
				CandidateIdentifier: delegate.Bytes(),
				VoterAmountFrozen:   big.NewInt(pool),
				TotalWeight:         new(big.Int).Set(snapshot.TotalWeight),
				SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
			}},
		},
	}
	r.NoError(p.writeEpochDrainCursor(activationCtx, sm, cursor))

	voters := []address.Address{alice, bob, carol, dave, eve}
	logs := drainCollectingVoterPayouts(t, activationCtx, sm, p, voters)
	paid := make(map[string]int, len(logs))
	for _, log := range logs {
		paid[log.Recipient]++
	}
	r.Equal(1, paid[alice.String()], "mixed native+LSD voter must be visited once")
	r.Equal(1, paid[bob.String()])
	r.Equal(1, paid[carol.String()])
	r.Zero(paid[dave.String()], "post-freeze transferee must not receive the frozen owner's share")
	r.Zero(paid[eve.String()], "post-freeze bucket owner must not participate in the frozen era")

	balances := accountBalances(t, sm, voters)
	for _, voter := range []address.Address{alice, bob, carol} {
		want := new(big.Int).Mul(big.NewInt(pool), weights[voter.String()])
		want.Div(want, totalWeight)
		r.Zero(want.Cmp(balances[voter.String()]),
			"voter %s must be paid from the pre-mutation oracle (got %s want %s)",
			voter, balances[voter.String()], want)
	}
	r.Zero(balances[dave.String()].Sign())
	r.Zero(balances[eve.String()].Sign())
}

func iip59BackfillDrainCtx(base context.Context, height uint64) context.Context {
	ctx := protocol.WithBlockCtx(base, protocol.BlockCtx{BlockHeight: height})
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{Caller: identityset.Address(0)})
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	return protocol.WithFeatureCtx(ctx)
}
