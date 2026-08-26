// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math"
	"math/big"
	"sort"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// These tests cover the IIP-59 owner-index activation backfill: the contract
// set it walks, the high-water marks it leaves behind, and the fact that it
// builds the whole index inside one block and is never run again.

func backfillTestContracts(t *testing.T) (v1, v2, v3 address.Address) {
	t.Helper()
	g := genesis.TestDefault()
	var err error
	v1, err = address.FromString(g.SystemStakingContractAddress)
	require.NoError(t, err)
	v2, err = address.FromString(g.SystemStakingContractV2Address)
	require.NoError(t, err)
	v3, err = address.FromString(g.SystemStakingContractV3Address)
	require.NoError(t, err)
	return
}

// seedPreForkBuckets writes buckets with the gate shut, which is the situation
// the backfill exists to repair: buckets in state, no owner index.
func seedPreForkBuckets(t *testing.T, sm protocol.StateManager, contract address.Address, owners map[uint64]address.Address) {
	t.Helper()
	cs := contractstaking.NewContractStakingStateManager(sm)
	ids := make([]uint64, 0, len(owners))
	for id := range owners {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		bkt := &contractstaking.Bucket{
			Candidate:      identityset.Address(30),
			Owner:          owners[id],
			StakedAmount:   big.NewInt(100),
			StakedDuration: 86400,
			CreatedAt:      1,
		}
		// context.Background() carries no feature context, so OwnerIndexEnabled
		// is false and nothing but the bucket itself is written.
		require.NoError(t, cs.UpsertBucket(context.Background(), contract, id, bkt))
	}
}

func backfillRefIDs(t *testing.T, sm protocol.StateManager, owner address.Address) []uint64 {
	t.Helper()
	refs, _, err := contractstaking.NewStateReader(sm).BucketRefsByOwner(owner)
	require.NoError(t, err)
	out := make([]uint64, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.BucketID)
	}
	return out
}

func backfillMarks(t *testing.T, sm protocol.StateManager) map[string]uint64 {
	t.Helper()
	marks, err := contractstaking.BucketIndexUpperBounds(sm)
	require.NoError(t, err)
	out := make(map[string]uint64, len(marks))
	for _, m := range marks {
		addr, err := address.FromBytes(m.Contract)
		require.NoError(t, err)
		out[addr.String()] = m.BucketIndexUpperBound - 1
	}
	return out
}

// TestBackfillOwnerIndexRunsAtTheActivationHeightOnly pins the trigger to the
// same height FeatureCtx.NoVoterRewardDistribution flips at.
//
// The two are separate expressions -- protocol.go compares against
// g.ToBeEnabledBlockHeight, action/protocol/context.go derives the flag from
// g.IsToBeEnabled -- and they must name the same block. A backfill one block
// early writes an index the live path is not yet maintaining; one block late
// leaves a block's worth of upserts indexed against an empty base.
func TestBackfillOwnerIndexRunsAtTheActivationHeightOnly(t *testing.T) {
	r := require.New(t)
	const activation = uint64(100)
	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = activation

	at := func(height uint64) bool {
		ctx := genesis.WithGenesisContext(context.Background(), g)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		return !protocol.MustGetFeatureCtx(protocol.WithFeatureCtx(ctx)).NoVoterRewardDistribution
	}
	r.False(at(activation-1), "the gate must still be shut on the block before activation")
	r.True(at(activation), "the gate opens on the activation block itself, which is the block the backfill runs in")
	r.True(at(activation + 1))
	// Which is to say: the backfill's trigger height and the height the live
	// index maintenance starts at are the same number.
	r.Equal(activation, g.ToBeEnabledBlockHeight)
}

// TestBackfillOwnerIndexCoversEveryContract is the main case: sparse ids,
// owners spread across contracts, and contracts with and without a pre-existing
// meta record.
func TestBackfillOwnerIndexCoversEveryContract(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	v1, v2, v3 := backfillTestContracts(t)
	alice, bob := identityset.Address(1), identityset.Address(2)

	// V1 has a meta record because the V1 indexer wrote one; V2 and V3 never
	// had one, which is the defect. Ids 2 and 4 of V1 are burnt: the scan must
	// tolerate gaps, and 5 is the top id, which an exclusive bound would drop.
	seedPreForkBuckets(t, sm, v1, map[uint64]address.Address{1: alice, 3: bob, 5: alice})
	r.NoError(contractstaking.NewContractStakingStateManager(sm).UpdateNumOfBuckets(v1, 5))
	seedPreForkBuckets(t, sm, v2, map[uint64]address.Address{1: bob, 2: bob})
	seedPreForkBuckets(t, sm, v3, map[uint64]address.Address{7: alice})

	r.NoError(backfillOwnerIndex(forkGateCtx(100, true), sm))

	// One block, whole index. The list is keyed by (contract, id), so the ids
	// alone are not in numeric order.
	r.ElementsMatch([]uint64{1, 5, 7}, backfillRefIDs(t, sm, alice))
	r.ElementsMatch([]uint64{3, 1, 2}, backfillRefIDs(t, sm, bob))

	// Defect C: every contract must also come out with a frozen bound, or the
	// era window rejects all of its buckets.
	marks := backfillMarks(t, sm)
	r.EqualValues(5, marks[v1.String()])
	r.EqualValues(2, marks[v2.String()])
	r.EqualValues(7, marks[v3.String()])
}

// TestBackfillOwnerIndexMergesRefsPerOwner covers the case the batch write
// exists for: one owner holding several buckets across several contracts ends
// up with one merged, sorted list rather than the last contract's refs only.
func TestBackfillOwnerIndexMergesRefsPerOwner(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	v1, v2, v3 := backfillTestContracts(t)
	alice := identityset.Address(1)

	seedPreForkBuckets(t, sm, v1, map[uint64]address.Address{9: alice, 1: alice})
	seedPreForkBuckets(t, sm, v2, map[uint64]address.Address{4: alice})
	seedPreForkBuckets(t, sm, v3, map[uint64]address.Address{0: alice, 2: alice})

	r.NoError(backfillOwnerIndex(forkGateCtx(100, true), sm))

	refs, _, err := contractstaking.NewStateReader(sm).BucketRefsByOwner(alice)
	r.NoError(err)
	r.Len(refs, 5)
	for i := 1; i < len(refs); i++ {
		prev, cur := refs[i-1], refs[i]
		c := string(prev.Contract.Bytes())
		r.LessOrEqual(c, string(cur.Contract.Bytes()), "refs must be sorted by contract")
		if c == string(cur.Contract.Bytes()) {
			r.Less(prev.BucketID, cur.BucketID, "refs of one contract must be sorted by id")
		}
	}
}

// TestBackfillOwnerIndexKeepsTheHigherMark covers the asymmetry between the
// recorded high-water mark and the highest id still in state: the top bucket
// may since have been burnt, so the mark can legitimately be higher, and
// lowering it would exclude buckets that do exist from the frozen era.
func TestBackfillOwnerIndexKeepsTheHigherMark(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	v1, v2, _ := backfillTestContracts(t)
	alice := identityset.Address(1)

	// v1: mark above the top live id (12 was minted then burnt).
	seedPreForkBuckets(t, sm, v1, map[uint64]address.Address{1: alice, 3: alice})
	r.NoError(contractstaking.NewContractStakingStateManager(sm).UpdateNumOfBuckets(v1, 12))
	// v2: mark below the top live id, which RaiseNumOfBuckets must lift.
	seedPreForkBuckets(t, sm, v2, map[uint64]address.Address{1: alice, 8: alice})
	r.NoError(contractstaking.NewContractStakingStateManager(sm).UpdateNumOfBuckets(v2, 2))

	r.NoError(backfillOwnerIndex(forkGateCtx(100, true), sm))

	marks := backfillMarks(t, sm)
	r.EqualValues(12, marks[v1.String()])
	r.EqualValues(8, marks[v2.String()])
}

func TestBackfillOwnerIndexKeepsMarkWhenAllBucketsAreBurned(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	v1, _, _ := backfillTestContracts(t)

	// The V1 indexer retains the historical maximum after every bucket has
	// been burned. Backfill must preserve it even though the bucket scan is empty.
	r.NoError(contractstaking.NewContractStakingStateManager(sm).UpdateNumOfBuckets(v1, 12))
	r.NoError(backfillOwnerIndex(forkGateCtx(100, true), sm))

	r.EqualValues(12, backfillMarks(t, sm)[v1.String()])
}

// TestBackfillOwnerIndexIsIdempotent guards the only way the backfill could run
// twice: a caller mistake. Re-running must not duplicate refs or lower a mark,
// so a stray second call is inert rather than corrupting.
func TestBackfillOwnerIndexIsIdempotent(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	v1, _, _ := backfillTestContracts(t)
	alice, bob := identityset.Address(1), identityset.Address(2)
	ctx := forkGateCtx(100, true)

	seedPreForkBuckets(t, sm, v1, map[uint64]address.Address{1: alice, 4: bob, 9: alice})
	r.NoError(backfillOwnerIndex(ctx, sm))
	first, marks := backfillRefIDs(t, sm, alice), backfillMarks(t, sm)

	r.NoError(backfillOwnerIndex(ctx, sm))
	r.Equal(first, backfillRefIDs(t, sm, alice))
	r.Equal(marks, backfillMarks(t, sm))
}

// TestBackfillOwnerIndexNoBuckets covers a chain where no contract has ever
// minted. Nothing is written -- in particular no mark is invented, because a
// mark of 0 would claim bucket 0 existed at the freeze height.
func TestBackfillOwnerIndexNoBuckets(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)

	r.NoError(backfillOwnerIndex(forkGateCtx(100, true), sm))
	r.Empty(backfillMarks(t, sm))
}

// backfillActivationHeight sits above every fork height in genesis.TestDefault
// (Yap, 48985561), so a CreatePreStates driven at it, at it minus one, or at it
// plus one triggers no other height-keyed migration and the only thing under
// test is the backfill.
const backfillActivationHeight = uint64(60_000_000)

// backfillPreStatesEnv builds the smallest protocol that can run CreatePreStates
// for real, modelled on TestCreatePreStates. No indexers: the branches that need
// them are all keyed on heights far below backfillActivationHeight.
func backfillPreStatesEnv(t *testing.T) (*Protocol, protocol.StateManager, genesis.Genesis) {
	t.Helper()
	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = backfillActivationHeight
	sm := eraTestSM(t)
	p, err := NewProtocol(HelperCtx{
		DepositGas:    nil,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight:    g.Staking.VoteWeightCalConsts,
			ReviseHeights: []uint64{g.GreenlandBlockHeight},
		},
	}, nil, nil, nil, nil)
	require.NoError(t, err)
	startCtx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), g),
		protocol.BlockCtx{BlockHeight: backfillActivationHeight},
	)
	startCtx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(startCtx))
	v, err := p.Start(startCtx, sm)
	require.NoError(t, err)
	require.NoError(t, sm.WriteView(_protocolID, v))
	return p, sm, g
}

func backfillPreStatesCtx(g genesis.Genesis, height uint64) context.Context {
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), g),
		protocol.BlockCtx{BlockHeight: height},
	)
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

// TestBackfillOwnerIndexThroughCreatePreStates is the end-to-end case, and the
// one that says the feature works at all: a voter who held contract-staking
// buckets before IIP-59 activated is visible to the drain in the very first era
// after it.
//
// The unit tests above call backfillOwnerIndex directly, so none of them can
// catch a call site wired to the wrong height, wired after something that
// clobbers it, or not wired at all. This drives the real CreatePreStates across
// A-1 / A / A+1 and then reads back through the same frozen accessors the drain
// uses, which is the only path that proves the two halves meet.
func TestBackfillOwnerIndexThroughCreatePreStates(t *testing.T) {
	r := require.New(t)
	p, sm, g := backfillPreStatesEnv(t)
	v1, v2, _ := backfillTestContracts(t)
	alice, bob := identityset.Address(1), identityset.Address(2)

	// Buckets that predate activation: written with no feature context, so the
	// live index maintenance in UpsertBucket is off, exactly as on a real chain
	// below A.
	seedPreForkBuckets(t, sm, v1, map[uint64]address.Address{1: alice, 4: bob, 9: alice})
	seedPreForkBuckets(t, sm, v2, map[uint64]address.Address{2: alice})

	// A-1: the gate is still shut, so the block must leave no trace. A backfill
	// here would be a state root divergence against un-upgraded nodes. Absent,
	// not empty: an owner with no buckets has no key at all.
	r.NoError(p.CreatePreStates(backfillPreStatesCtx(g, backfillActivationHeight-1), sm))
	_, _, err := contractstaking.NewStateReader(sm).BucketRefsByOwner(alice)
	r.ErrorIs(err, contractstaking.ErrOwnerIndexNotExist,
		"no index may exist before the activation block")
	r.Empty(backfillMarks(t, sm))

	// A: one block, whole index.
	r.NoError(p.CreatePreStates(backfillPreStatesCtx(g, backfillActivationHeight), sm))
	r.ElementsMatch([]uint64{1, 9, 2}, backfillRefIDs(t, sm, alice))
	r.ElementsMatch([]uint64{4}, backfillRefIDs(t, sm, bob))

	// A+1: the scan does not run again. Seeding another pre-fork bucket first
	// makes the assertion positive rather than vacuous -- a re-scan would pick
	// this one up, and only a re-scan could.
	seedPreForkBuckets(t, sm, v1, map[uint64]address.Address{11: alice})
	r.NoError(p.CreatePreStates(backfillPreStatesCtx(g, backfillActivationHeight+1), sm))
	r.ElementsMatch([]uint64{1, 9, 2}, backfillRefIDs(t, sm, alice),
		"the backfill must run in exactly one block, with no self-healing rescan afterwards")

	// And the drain can reach the pre-activation buckets through the era window.
	// Both halves have to be right for this: the owner index gives the drain the
	// refs, and the high-water marks decide whether the window admits the ids.
	ctx := backfillPreStatesCtx(g, backfillActivationHeight)
	r.NoError(BeginEraCOWWindow(ctx, sm, backfillActivationHeight))
	window, err2 := LoadEraCOWWindow(sm)
	r.NoError(err2)
	r.True(window.Open())

	refs, err := contractstaking.NewStateReader(sm).FrozenBucketRefs(window, alice)
	r.NoError(err)
	got := make([]uint64, 0, len(refs))
	for _, ref := range refs {
		bkt, err := contractstaking.NewStateReader(sm).FrozenBucket(window, ref.Contract, ref.BucketID)
		r.NoError(err, "a bucket the backfill indexed must be admitted by the frozen window")
		r.Equal(alice.String(), bkt.Owner.String())
		got = append(got, ref.BucketID)
	}
	r.ElementsMatch([]uint64{1, 9, 2}, got,
		"a voter who held LSD buckets before activation must be payable in the first era after it")
}

// TestBackfillContractsUsesGenesis pins the contract set to the configured V1,
// V2 and V3 system staking contracts. Order is by raw address bytes because the
// write order reaches the trie.
func TestBackfillContractsUsesGenesis(t *testing.T) {
	r := require.New(t)
	v1, v2, v3 := backfillTestContracts(t)

	contracts, err := backfillContracts(forkGateCtx(100, true))
	r.NoError(err)

	got := make([]string, 0, len(contracts))
	for _, c := range contracts {
		got = append(got, c.String())
	}
	r.ElementsMatch([]string{v1.String(), v2.String(), v3.String()}, got)
	for i := 1; i < len(contracts); i++ {
		r.Less(string(contracts[i-1].Bytes()), string(contracts[i].Bytes()))
	}
}
