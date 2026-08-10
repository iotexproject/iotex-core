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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// TestSplitCommission covers the pure basis-points helper. The rewarding
// path relies on splitCommission truncating in favour of the voter pool and
// clamping malformed on-chain rates so a corrupted DelegateProfile value
// cannot over-pay commission.
func TestSplitCommission(t *testing.T) {
	cases := []struct {
		name       string
		total      *big.Int
		bps        uint64
		wantComm   *big.Int
		wantVoters *big.Int
	}{
		{"zero total", big.NewInt(0), 1000, big.NewInt(0), big.NewInt(0)},
		{"nil total", nil, 1000, big.NewInt(0), big.NewInt(0)},
		{"zero bps", big.NewInt(1000), 0, big.NewInt(0), big.NewInt(1000)},
		{"ten percent", big.NewInt(1000), 1000, big.NewInt(100), big.NewInt(900)},
		{"truncation favours voters", big.NewInt(3), 1000, big.NewInt(0), big.NewInt(3)},
		{"exactly 100 percent clamps", big.NewInt(1000), 10_000, big.NewInt(1000), big.NewInt(0)},
		{"over 100 percent clamps", big.NewInt(1000), 20_000, big.NewInt(1000), big.NewInt(0)},
		{"one bp", big.NewInt(10_000), 1, big.NewInt(1), big.NewInt(9_999)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			comm, voters := splitCommission(tc.total, tc.bps)
			r.Equal(0, comm.Cmp(tc.wantComm), "commission: got %s want %s", comm.String(), tc.wantComm.String())
			r.Equal(0, voters.Cmp(tc.wantVoters), "voter pool: got %s want %s", voters.String(), tc.wantVoters.String())
		})
	}
}

// TestSplitDelegateEpochReward covers the fallback branches that route the
// full amount to commission (voter share = 0), and the happy-path split.
// Fallback cases must return (amount, 0) so GrantEpochReward's caller runs
// the legacy per-delegate grant unchanged.
func TestSplitDelegateEpochReward(t *testing.T) {
	amount := big.NewInt(1_000)

	t.Run("fork off", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, _ := newVoterRewardCtx(t, false /* iip59On */)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(amount))
		r.Equal(0, v.Sign())
	})

	t.Run("nil candidate", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, nil, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(amount))
		r.Equal(0, v.Sign())
	})

	t.Run("zero amount", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, _ := newVoterRewardCtx(t, true)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, big.NewInt(0))
		r.NoError(err)
		r.Equal(0, c.Sign())
		r.Equal(0, v.Sign())
	})

	t.Run("negative amount rejected", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, _ := newVoterRewardCtx(t, true)
		_, _, err := p.splitDelegateEpochReward(ctx, sm, cand, big.NewInt(-1))
		r.Error(err)
	})

	t.Run("no snapshot fallback", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, _ := newVoterRewardCtx(t, true)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Zero(c.Cmp(amount))
		r.Zero(v.Sign())
	})

	t.Run("unregistered defaults to all owner", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
		writeSnapshot(t, sm, candAddr, false, _basisPointsDenom, []voterEntry{{identityset.Address(3), big.NewInt(100)}})
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Zero(c.Cmp(amount))
		r.Zero(v.Sign())
	})

	t.Run("empty voters fallback", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
		writeSnapshot(t, sm, candAddr, true, 2000, nil)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Zero(c.Cmp(big.NewInt(200)))
		r.Zero(v.Cmp(big.NewInt(800)))
	})

	t.Run("happy path 20 percent commission", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
		writeSnapshot(t, sm, candAddr, true, 2000, []voterEntry{{identityset.Address(3), big.NewInt(100)}})
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(big.NewInt(200)))
		r.Equal(0, v.Cmp(big.NewInt(800)))
	})
}

// The three tests that used to live here drove distributeVoterOnly, the
// candidate-major payout that walked a frozen entry list. That function is
// gone: the drain is voter-major and pays each voter once for their whole era
// entitlement. What survives is the routing contract those tests pinned --
// custom reward destination, and compound taking priority over it -- so they
// are re-expressed against payVoterCombined, which is where routing now lives.

// newRoutingShares builds a one-delegate share set for a routing test. Routing
// is indifferent to how the number was derived; it only moves the total.
func newRoutingShares(delegate address.Address, amount *big.Int) (voterShareSet, voterShareInputs) {
	work := epochDrainDelegateWork{
		CandidateIdentifier: delegate.Bytes(),
		VoterAmountFrozen:   new(big.Int).Set(amount),
		FreezeHeight:        iip59FixtureFreezeHeight,
		SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
	}
	delegates := []epochDrainDelegateWork{work}
	return voterShareSet{
			shares: []voterDelegateShare{{
				delegateIndex: 0,
				candidate:     delegate,
				weight:        big.NewInt(1),
				share:         new(big.Int).Set(amount),
			}},
			total: new(big.Int).Set(amount),
		}, voterShareInputs{
			delegates:   delegates,
			byCandidate: delegateWorkIndex(delegates),
			payable:     []bool{true},
			distributed: []*big.Int{new(big.Int)},
		}
}

// TestPayVoterCombinedCustomRewardDestination pins that a voter who has
// registered a reward destination is credited there rather than at their own
// address, and that the DelegateDistributed row records both.
func TestPayVoterCombinedCustomRewardDestination(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)
	voter := identityset.Address(3)
	recipient := identityset.Address(4)
	r.NoError(p.putState(ctx, sm, voterRewardDestinationKey(voter), &voterRewardDestination{
		recipient: recipient, updatedHeight: 100,
	}))

	routing, err := p.resolveVoterRouting(ctx, sm)
	r.NoError(err)
	shares, in := newRoutingShares(candAddr, big.NewInt(777))
	payout, err := p.payVoterCombined(ctx, sm, routing, in, voter, shares, &iip59RouteDurations{})
	r.NoError(err)
	r.Equal(recipient.String(), payout.recipient.String())
	r.Zero(payout.amount.Cmp(big.NewInt(777)))
	r.Zero(payout.compoundBucketID)

	voterAccount, err := accountutil.LoadAccount(sm, voter)
	r.NoError(err)
	r.Zero(voterAccount.Balance.Sign(), "the voter's own balance must stay untouched")
	recipientAccount, err := accountutil.LoadAccount(sm, recipient)
	r.NoError(err)
	r.Zero(recipientAccount.Balance.Cmp(big.NewInt(777)))

	// A directly credited payout is an outflow from the rewarding fund and
	// must produce a transaction log naming the destination, not the voter.
	txLog := voterTransactionLog(payout)
	r.NotNil(txLog)
	r.Equal(recipient.String(), txLog.Recipient)
	r.Zero(txLog.Amount.Cmp(big.NewInt(777)))

	logs := make([]delegateChunkLog, 1)
	recordVoterPayout(logs, payout)
	r.Equal([]address.Address{voter}, logs[0].voters)
	r.Equal([]address.Address{recipient}, logs[0].recipients)
	r.Zero(logs[0].paid.Cmp(big.NewInt(777)))
}

// TestPayVoterCombinedCompoundOverridesCustomRewardDestination pins the
// priority order: an eligible auto-deposit bucket absorbs the payout even when
// the voter has also registered a custom destination. Compounding is not a
// transfer, so it must leave the destination account at zero and emit no
// transaction log -- the outflow is settled once per block instead.
func TestPayVoterCombinedCompoundOverridesCustomRewardDestination(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    100,
		BlockTimeStamp: time.Unix(100, 0).UTC(),
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		Tip: protocol.TipInfo{Height: 99},
	})
	g := genesis.TestDefault()

	csm, err := staking.NewCandidateStateManager(sm)
	r.NoError(err)
	delegates, err := staking.TestOnlySeedPerfBenchState(ctx, csm, staking.TestOnlyPerfBenchSpec{
		NumDelegates:            1,
		NumVoters:               1,
		DelegateSelfStake:       big.NewInt(1_000_000),
		VoterStake:              big.NewInt(1_000),
		VoterStakedDurationDays: 30,
		VoteWeightCalConsts:     g.Staking.VoteWeightCalConsts,
	})
	r.NoError(err)
	r.NoError(csm.Commit(ctx))
	r.Len(delegates, 1)
	candAddr := delegates[0]
	voter := staking.TestOnlyPerfBenchVoterAddress(0)
	destination := identityset.Address(8)

	csr, err := staking.ConstructBaseView(sm)
	r.NoError(err)
	buckets, _, err := csr.NativeBuckets()
	r.NoError(err)
	var compoundBucket *staking.VoteBucket
	for _, bucket := range buckets {
		if address.Equal(bucket.Owner, voter) {
			compoundBucket = bucket
			break
		}
	}
	r.NotNil(compoundBucket)
	r.True(autodeposit.IsBucketEligibleForCompound(compoundBucket, voter))
	initialBucketAmount := new(big.Int).Set(compoundBucket.StakedAmount)

	bridge, err := autodeposit.New(identityset.Address(0).String())
	r.NoError(err)
	p.autoDepositBridge = bridge
	bucketReader := &registeredBucketReader{voter: voter, bucketID: compoundBucket.Index}
	p.autoDepositBucketReaderFactory = func(autodeposit.SlotReader) autodeposit.BucketReader {
		return bucketReader
	}
	r.NoError(p.putState(ctx, sm, voterRewardDestinationKey(voter), &voterRewardDestination{
		recipient: destination, updatedHeight: 100,
	}))

	routing, err := p.resolveVoterRouting(ctx, sm)
	r.NoError(err)
	shares, in := newRoutingShares(candAddr, big.NewInt(777))
	payout, err := p.payVoterCombined(ctx, sm, routing, in, voter, shares, &iip59RouteDurations{})
	r.NoError(err)
	r.Equal(1, bucketReader.callCount)
	r.Equal(compoundBucket.Index, payout.compoundBucketID)
	r.Zero(payout.amount.Cmp(big.NewInt(777)))

	updatedCSR, err := staking.ConstructBaseView(sm)
	r.NoError(err)
	updatedBucket, err := updatedCSR.NativeBucket(compoundBucket.Index)
	r.NoError(err)
	r.Zero(updatedBucket.StakedAmount.Cmp(new(big.Int).Add(initialBucketAmount, big.NewInt(777))))

	destinationAccount, err := accountutil.LoadAccount(sm, destination)
	r.NoError(err)
	r.Zero(destinationAccount.Balance.Sign(), "compound must take priority over the custom destination")
	r.Nil(voterTransactionLog(payout), "compound payout must not emit a direct account transfer")
}

const delegateDistributedDestinationTestABI = `[{"anonymous":false,"inputs":[
	{"indexed":true,"name":"epoch","type":"uint64"},
	{"indexed":true,"name":"delegate","type":"address"},
	{"indexed":false,"name":"rewardAddr","type":"address"},
	{"indexed":false,"name":"totalCommission","type":"uint256"},
	{"indexed":false,"name":"totalVoterPool","type":"uint256"},
	{"indexed":false,"name":"snapshotHash","type":"bytes32"},
	{"indexed":false,"name":"voters","type":"address[]"},
	{"indexed":false,"name":"recipients","type":"address[]"},
	{"indexed":false,"name":"amounts","type":"uint256[]"},
	{"indexed":false,"name":"compoundBucketIds","type":"uint64[]"}],
	"name":"DelegateDistributed","type":"event"}]`

type voterEntry struct {
	addr   address.Address
	weight *big.Int
}

func writeSnapshot(
	t *testing.T,
	sm protocol.StateManager,
	candAddr address.Address,
	registered bool,
	epochBps uint64,
	voters []voterEntry,
) {
	t.Helper()
	// The snapshot no longer carries a per-voter list; the frozen denominator
	// is the sum, which for these fixtures is what candidate.Votes would have
	// been at the boundary. Callers still pass voters because the amounts are
	// what make each fixture's expected split legible.
	totalWeight := new(big.Int)
	for _, v := range voters {
		totalWeight.Add(totalWeight, v.weight)
	}
	snap := &staking.CandidatePollSnapshot{
		OnchainRewardEnabled:       true,
		Registered:                 registered,
		BlockCommissionBasisPoints: epochBps,
		EpochCommissionBasisPoints: epochBps,
		TotalWeight:                totalWeight,
	}
	require.NoError(t, staking.TestOnlyPutPollSnapshotFor(sm, candAddr, snap))
}

// distributionMetadata reads back the two numbers Phase A freezes into a work
// item. It returns what the frozen snapshot recorded rather than recomputing,
// because that is what the cursor carries and what the drain divides by.
func distributionMetadata(
	t *testing.T,
	sm protocol.StateReader,
	candAddr address.Address,
) (*big.Int, hash.Hash256) {
	t.Helper()
	snapshot, err := staking.PollSnapshotFor(sm, candAddr)
	require.NoError(t, err)
	return snapshot.TotalWeight, snapshot.SnapshotHash
}

// testBlockIntervalSwitchHeight is the height at which testBlocksToDuration
// changes block interval. It has to sit above iip59FixtureFreezeHeight and
// above the height an ordinary drain runs at, but below the far height the
// evalHeight test drains at, so that two drains of the same frozen era land on
// opposite sides of it. That gap is the only thing that lets a test distinguish
// "evaluated at the freeze height" from "evaluated at the current block", and
// every other test is unaffected because it never drains above this height.
const testBlockIntervalSwitchHeight = uint64(1000)

// testBlocksToDuration mirrors the shape of the production converter
// (chainservice.Builder.blocksToDurationFn): the same block span maps to a
// different wall-clock duration depending on the height it is viewed at,
// because a hardfork changed the block interval. A test protocol built with a
// nil converter would nil-panic on any non-timestamp contract bucket, and one
// built with a height-insensitive converter would silently pass the evalHeight
// tests no matter which height the drain used.
func testBlocksToDuration(start, end, viewAt uint64) time.Duration {
	if viewAt < testBlockIntervalSwitchHeight {
		return time.Duration(end-start) * 5 * time.Second
	}
	return time.Duration(end-start) * time.Second
}

// newVoterRewardCtx wires the minimum context splitDelegateEpochReward reads:
// a StateManager, registered rolldpos+staking protocols, and feature ctx
// toggled by iip59On.
func newVoterRewardCtx(
	t *testing.T,
	iip59On bool,
) (context.Context, protocol.StateManager, *Protocol, *state.Candidate, address.Address) {
	t.Helper()
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	g := genesis.TestDefault()
	if iip59On {
		g.ToBeEnabledBlockHeight = 1
		g.Rewarding.HermesRewardVaultAddresses = []string{identityset.Address(2).String()}
	} else {
		g.ToBeEnabledBlockHeight = 1_000_000_000
	}

	registry := protocol.NewRegistry()
	rp := rolldpos.NewProtocol(g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs)
	r.NoError(rp.Register(registry))

	stakingCfg := &staking.BuilderConfig{
		Staking: g.Staking,
		Revise: staking.ReviseConfig{
			VoteWeight: g.VoteWeightCalConsts,
		},
	}
	stakingProtocol, err := staking.NewProtocol(
		staking.HelperCtx{}, stakingCfg, testBlocksToDuration, nil, nil, nil,
	)
	r.NoError(err)
	r.NoError(stakingProtocol.Register(registry))

	p := NewProtocol(g.Rewarding)
	r.NoError(p.Register(registry))

	candAddr := identityset.Address(1)
	cand := &state.Candidate{
		Identity:      candAddr.String(),
		Address:       identityset.Address(9).String(),
		RewardAddress: identityset.Address(2).String(),
		Votes:         big.NewInt(1_000_000),
	}
	r.NoError(staking.TestOnlyPutCandidateRewardAddress(
		sm, candAddr, candAddr, identityset.Address(2), false,
	))

	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithRegistry(ctx, registry)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 100})
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{Caller: identityset.Address(0)})
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithFeatureCtx(ctx)

	view, err := stakingProtocol.Start(ctx, sm)
	r.NoError(err)
	r.NoError(sm.WriteView("staking", view))

	return ctx, sm, p, cand, candAddr
}

// TestDistributeVoterOnly_WindowedDeterminism used to prove that splitting one
// delegate's entry list across blocks produced identical payouts. Chunk
// invariance is now a property of the whole voter-major drain rather than of
// one delegate's window, and is asserted end-to-end by
// TestChunkedDrain_InvariantAcrossChunkSizes.

// fakeBucketReader satisfies autodeposit.BucketReader with a canned response
// for use in the option-wiring test below.
type fakeBucketReader struct{ callCount int }

func (f *fakeBucketReader) LookupBucket(address.Address) (uint64, bool, error) {
	f.callCount++
	return 0, false, errors.New("unused")
}

type registeredBucketReader struct {
	voter     address.Address
	bucketID  uint64
	callCount int
}

func (r *registeredBucketReader) LookupBucket(voter address.Address) (uint64, bool, error) {
	r.callCount++
	if address.Equal(voter, r.voter) {
		return r.bucketID, true, nil
	}
	return 0, false, nil
}

// TestProtocolOptions verifies WithAutoDepositBridge / WithAutoDepositBucketReader
// install onto the Protocol so downstream distributeVoterOnly can consume
// them.
func TestProtocolOptions(t *testing.T) {
	r := require.New(t)
	g := genesis.TestDefault()
	bridge, err := autodeposit.New(identityset.Address(0).String())
	r.NoError(err)

	fake := &fakeBucketReader{}
	factoryCalled := false
	factory := func(autodeposit.SlotReader) autodeposit.BucketReader {
		factoryCalled = true
		return fake
	}

	p := NewProtocol(g.Rewarding, WithAutoDepositBridge(bridge), WithAutoDepositBucketReader(factory))
	r.NotNil(p.autoDepositBridge)
	r.NotNil(p.autoDepositBucketReaderFactory)

	// Exercise the seam so the coverage on resolveAutoDepositBucketReader is real.
	got, err := p.resolveAutoDepositBucketReader(nil)
	r.NoError(err)
	r.Same(fake, got)
	r.True(factoryCalled)
}
