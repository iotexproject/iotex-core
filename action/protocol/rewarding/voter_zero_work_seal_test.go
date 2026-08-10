// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// ------------------------------------------------------------------- R6 --

// TestZeroWorkEraSealsItsWindow is the R6 regression.
//
// An era boundary always opens a copy-on-write window; the only normal-path
// seal used to sit at drain completion. An era that produced no payable work
// wrote no cursor entries and returned before reaching it, so nothing ever
// sealed the window. The copy-on-write hooks then stayed armed on every bucket
// write indefinitely -- an unbounded, permanent cost imposed by an era that did
// no work at all -- and the next boundary would find a window already open.
//
// The seal has to be driven by consensus-visible state on a block every node
// agrees on. It is: whether the boundary produced entries is a function of
// committed state, and this runs inside GrantEpochReward on the epoch's last
// block, so every node seals in the same block or none does.
func TestZeroWorkEraSealsItsWindow(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)

	var seed hash.Hash256
	copy(seed[:], []byte{0x11, 0x22, 0x33})
	r.NoError(p.persistDrainCursor(ctx, sm, 1, seed, iip59FixtureFreezeHeight, nil))

	window, err := staking.EraCOWWindow(sm)
	r.NoError(err)
	r.False(window.Open(),
		"an era boundary with no payable work must seal its own window instead of leaking it")

	// And no cursor was invented on the way: a sealed window with a live cursor
	// would be worse than the leak, because the drain would then read frozen
	// state through a window that no longer holds the copies.
	cursor, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.Nil(cursor, "a zero-work era must not materialize a cursor")
}

// TestZeroWorkSealIsIdempotent pins that the new seal is safe on the path where
// no window was ever opened -- a boundary before activation, or one already
// sealed by a completed drain in the same block.
func TestZeroWorkSealIsIdempotent(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	var seed hash.Hash256
	r.NoError(p.persistDrainCursor(ctx, sm, 1, seed, iip59FixtureFreezeHeight, nil))
	r.NoError(p.persistDrainCursor(ctx, sm, 2, seed, iip59FixtureFreezeHeight, nil))

	window, err := staking.EraCOWWindow(sm)
	r.NoError(err)
	r.False(window.Open())
}

// ------------------------------------------------------------------- R7 --

// TestCompoundIntoNativeBucketZero is the R7 regression.
//
// Native bucket indices start at 0 and index 0 is a perfectly ordinary bucket.
// The payout used to record "was this compounded" as compoundBucketID != 0, so
// a voter whose auto-deposit bucket is index 0 compounded successfully and was
// then reported as a direct credit: a spurious CLAIM_FROM_REWARDING_FUND
// transaction log for tokens that never left the rewarding fund, the amount
// missing from the block's compound outflow, and a DelegateDistributed row
// telling off-chain consumers the voter was paid at their reward address when
// they were not.
func TestCompoundIntoNativeBucketZero(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    100,
		BlockTimeStamp: time.Unix(100, 0).UTC(),
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		Tip: protocol.TipInfo{Height: 99},
	})
	testdb.AllowRevert(sm.(*mock_chainmanager.MockStateManager))

	// The first native bucket ever planted takes index 0, which is the whole
	// point: this must be a real bucket 0, not one relabelled by the test.
	voter := identityset.Address(3)
	bucketID, err := staking.TestOnlySeedNativeVoterBucket(
		sm, candAddr, voter, big.NewInt(1_000), 30, time.Unix(0, 0).UTC(), true,
	)
	r.NoError(err)
	r.Zero(bucketID, "the fixture must exercise a genuine native bucket 0")

	csr, err := staking.ConstructBaseView(sm)
	r.NoError(err)
	bucket, err := csr.NativeBucket(0)
	r.NoError(err)
	r.True(autodeposit.IsBucketEligibleForCompound(bucket, voter))
	stakedBefore := new(big.Int).Set(bucket.StakedAmount)

	// The candidate's running vote total has to already include this bucket's
	// weight. AddDepositForCompound recomputes by subtracting the old weight
	// before adding the new one, so a candidate seeded with zero votes would
	// underflow and take the (correct, but not what this test is measuring)
	// chain-determined degrade path instead of compounding.
	g := genesis.TestDefault()
	csm, err := staking.NewCandidateStateManager(sm)
	r.NoError(err)
	cand := csm.GetByIdentifier(candAddr)
	r.NotNil(cand)
	cand = cand.Clone()
	cand.Votes = staking.CalculateVoteWeight(g.VoteWeightCalConsts, bucket, false)
	r.NoError(csm.Upsert(cand))
	r.NoError(csm.DebitBucketPool(bucket.StakedAmount, true))
	r.NoError(csm.Commit(ctx))

	bridge, err := autodeposit.New(identityset.Address(0).String())
	r.NoError(err)
	p.autoDepositBridge = bridge
	p.autoDepositBucketReaderFactory = func(autodeposit.SlotReader) autodeposit.BucketReader {
		return &registeredBucketReader{voter: voter, bucketID: 0}
	}
	routing, err := p.resolveVoterRouting(ctx, sm)
	r.NoError(err)

	amount := big.NewInt(777)
	shares, in := newRoutingShares(candAddr, amount)
	payout, err := p.payVoterCombined(ctx, sm, routing, in, voter, shares, &iip59RouteDurations{})
	r.NoError(err)

	// The discriminator is the flag, not the id.
	r.True(payout.compounded, "a compound into bucket 0 must be recorded as compounded")
	r.Zero(payout.compoundBucketID)

	// The tokens really did go into the bucket.
	updatedCSR, err := staking.ConstructBaseView(sm)
	r.NoError(err)
	updated, err := updatedCSR.NativeBucket(0)
	r.NoError(err)
	r.Zero(updated.StakedAmount.Cmp(new(big.Int).Add(stakedBefore, amount)))

	// So there is no transfer out of the rewarding fund, and therefore must be
	// no transaction log claiming one. This is the assertion that fails when
	// bucket 0 is confused with the "not compounded" sentinel.
	r.Nil(voterTransactionLog(payout),
		"a compound into bucket 0 must not emit a CLAIM_FROM_REWARDING_FUND log")

	// And the emitted row carries the flag, so an off-chain consumer can tell
	// bucket 0 from "not compounded" without guessing.
	rows := make([]delegateChunkLog, 1)
	recordVoterPayout(rows, payout)
	r.Equal([]bool{true}, rows[0].compounded)
	r.Equal([]uint64{0}, rows[0].compoundBucketIDs)
	r.Equal([]address.Address{voter}, rows[0].voters)
}

// TestNonCompoundedPayoutCarriesBucketZeroWithFalseFlag is the counterpart: a
// voter who was NOT compounded also records bucket id 0, because there is no
// bucket. The two cases are byte-identical in the id column and are separable
// only by the flag -- which is exactly why the flag had to be added.
func TestNonCompoundedPayoutCarriesBucketZeroWithFalseFlag(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)
	voter := identityset.Address(3)

	routing, err := p.resolveVoterRouting(ctx, sm)
	r.NoError(err)
	shares, in := newRoutingShares(candAddr, big.NewInt(500))
	payout, err := p.payVoterCombined(ctx, sm, routing, in, voter, shares, &iip59RouteDurations{})
	r.NoError(err)

	r.False(payout.compounded)
	r.Zero(payout.compoundBucketID, "an uncompounded payout leaves the id at its zero value")
	r.NotNil(voterTransactionLog(payout), "a direct credit is a real transfer and must be logged")

	rows := make([]delegateChunkLog, 1)
	recordVoterPayout(rows, payout)
	r.Equal([]bool{false}, rows[0].compounded)
	r.Equal([]uint64{0}, rows[0].compoundBucketIDs)
}
