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
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// --------------------------------------------------------------- R5 unit --

// receiptStatusError is a stand-in for staking's unexported *handleError. What
// the classifier reads off such an error is its receipt status, and that is the
// entire contract this fake needs to honour. It deliberately does NOT implement
// Unwrap, mirroring *handleError, so a classifier that reached for the wrapped
// sentinel instead of the status would fail here exactly as it does in
// production.
type receiptStatusError struct {
	msg    string
	status uint64
}

func (e *receiptStatusError) Error() string         { return e.msg }
func (e *receiptStatusError) ReceiptStatus() uint64 { return e.status }

// TestCompoundErrorClassification walks the enumeration in the comment above
// compoundErrorIsChainDetermined and pins each row to its class.
//
// The asymmetry being pinned is the point. Misclassifying an infrastructure
// error as chain-determined lets one node degrade where another halts, which is
// two state roots for one block. Misclassifying a chain-determined error as
// infrastructure only stalls the drain, which is visible and fixable. So every
// row that is not positively known to be derivable from committed state must
// land in the halting class, including a bare error with no classification at
// all.
func TestCompoundErrorClassification(t *testing.T) {
	r := require.New(t)

	degrade := []struct {
		row string
		err error
	}{
		{"row 10: self-stake role changed since the freeze",
			errors.Wrap(staking.ErrCompoundSelfStakeRoleChanged, "bucket=7")},
		{"row 7: bucket owner mismatch",
			errors.Wrapf(staking.ErrCompoundBucketOwnerMismatch, "bucket=7 voter=x")},
		{"rows 12-14: vote accumulator drift",
			errors.Wrap(action.ErrInvalidAmount, "staking: subtract vote for candidate")},
		{"row 5: bucket genuinely absent",
			errors.Wrap(&receiptStatusError{"invalid bucket index",
				uint64(iotextypes.ReceiptStatus_ErrInvalidBucketIndex)}, "staking: fetch bucket 7")},
		{"row 8: candidate genuinely absent",
			errors.Wrap(&receiptStatusError{"candidate does not exist",
				uint64(iotextypes.ReceiptStatus_ErrCandidateNotExist)}, "staking: candidate missing")},
		{"row 15: candidate collision on upsert",
			&receiptStatusError{"candidate conflict",
				uint64(iotextypes.ReceiptStatus_ErrCandidateConflict)}},
	}
	for _, c := range degrade {
		r.True(compoundErrorIsChainDetermined(c.err),
			"%s must degrade to a direct credit", c.row)
	}

	halt := []struct {
		row string
		err error
	}{
		{"row 4: csm construction / view read failure",
			errors.Wrap(state.ErrStateNotExist, "staking: build csm for compound")},
		{"row 6: unclassified bucket read failure keeps the generic status",
			errors.Wrap(&receiptStatusError{"fetch bucket failed",
				uint64(iotextypes.ReceiptStatus_Failure)}, "staking: fetch bucket 7")},
		{"row 9: endorsement manager read failure",
			errors.New("staking: self-stake check for compound: read view")},
		{"row 11: updateBucket write failure",
			errors.New("staking: update compound bucket 7: put state")},
		{"row 16: putCandidate write failure",
			errors.New("staking: upsert candidate: put state")},
		{"row 17: bucket pool debit failure",
			errors.New("staking: debit bucket pool for compound")},
		{"rows 1-3: caller-contract guards",
			errors.New("staking: non-positive compound amount")},
		{"an unclassified success status is not a verdict either",
			&receiptStatusError{"weird", uint64(iotextypes.ReceiptStatus_Success)}},
	}
	for _, c := range halt {
		r.False(compoundErrorIsChainDetermined(c.err),
			"%s must halt the chunk, not degrade", c.row)
	}

	r.False(compoundErrorIsChainDetermined(nil))
}

// TestCompoundErrorClassificationMatchesStakingContract drives the exported
// staking entry point instead of manufacturing errors in rewarding. It is the
// cross-package contract test for the classifier above: if staking changes a
// sentinel, receipt status, or wrapping rule, the corresponding classification
// assertion fails here.
func TestCompoundErrorClassificationMatchesStakingContract(t *testing.T) {
	t.Run("caller guards halt", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)

		for _, tc := range []struct {
			name   string
			voter  address.Address
			amount *big.Int
		}{
			{name: "nil voter", amount: big.NewInt(1)},
			{name: "nil amount", voter: f.voter},
			{name: "zero amount", voter: f.voter, amount: new(big.Int)},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := sp.AddDepositForCompound(
					f.ctx, f.sm, tc.voter, f.bucket.Index, tc.amount, staking.FrozenSelfStake{},
				)
				require.Error(t, err)
				require.False(t, compoundErrorIsChainDetermined(err))
			})
		}
	})

	t.Run("missing bucket degrades with staking receipt", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)

		err := sp.AddDepositForCompound(
			f.ctx, f.sm, f.voter, f.bucket.Index+1_000_000, big.NewInt(1), staking.FrozenSelfStake{},
		)
		require.Error(t, err)
		var receiptErr staking.ReceiptError
		require.ErrorAs(t, err, &receiptErr)
		require.Equal(t, uint64(iotextypes.ReceiptStatus_ErrInvalidBucketIndex), receiptErr.ReceiptStatus())
		require.True(t, compoundErrorIsChainDetermined(err))
	})

	t.Run("owner mismatch degrades with staking sentinel", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)

		err := sp.AddDepositForCompound(
			f.ctx, f.sm, identityset.Address(20), f.bucket.Index, big.NewInt(1), staking.FrozenSelfStake{},
		)
		require.ErrorIs(t, err, staking.ErrCompoundBucketOwnerMismatch)
		require.True(t, compoundErrorIsChainDetermined(err))
	})

	t.Run("missing candidate degrades with staking receipt", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)
		missingCandidate := identityset.Address(20)
		bucketID, err := staking.TestOnlySeedNativeVoterBucket(
			f.sm, missingCandidate, f.voter, big.NewInt(1_000), 30, time.Unix(100, 0).UTC(), true,
		)
		require.NoError(t, err)

		err = sp.AddDepositForCompound(
			f.ctx, f.sm, f.voter, bucketID, big.NewInt(1), staking.FrozenSelfStake{},
		)
		require.Error(t, err)
		var receiptErr staking.ReceiptError
		require.ErrorAs(t, err, &receiptErr)
		require.Equal(t, uint64(iotextypes.ReceiptStatus_ErrCandidateNotExist), receiptErr.ReceiptStatus())
		require.True(t, compoundErrorIsChainDetermined(err))
	})

	t.Run("self stake role change degrades with staking sentinel", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)

		err := sp.AddDepositForCompound(
			f.ctx, f.sm, f.voter, f.bucket.Index, big.NewInt(1), staking.FrozenSelfStake{
				FreezeHeight: iip59FixtureFreezeHeight,
				BucketIdx:    f.bucket.Index,
			},
		)
		require.ErrorIs(t, err, staking.ErrCompoundSelfStakeRoleChanged)
		require.True(t, compoundErrorIsChainDetermined(err))
	})

	t.Run("candidate vote drift degrades with staking sentinel", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)
		csm, err := staking.NewCandidateStateManagerWithContext(f.ctx, f.sm)
		require.NoError(t, err)
		candidate := csm.GetByIdentifier(f.delegate)
		require.NotNil(t, candidate)
		candidate.Votes = new(big.Int)
		require.NoError(t, csm.Upsert(candidate))
		require.NoError(t, csm.Commit(f.ctx))

		err = sp.AddDepositForCompound(
			f.ctx, f.sm, f.voter, f.bucket.Index, big.NewInt(1), staking.FrozenSelfStake{},
		)
		require.ErrorIs(t, err, action.ErrInvalidAmount)
		require.True(t, compoundErrorIsChainDetermined(err))
	})

	t.Run("candidate conflict degrades with staking receipt", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)
		failing := &failingCandidatePutStateManager{
			StateManager: f.sm,
			err:          action.ErrInvalidCanName,
		}

		err := sp.AddDepositForCompound(
			f.ctx, failing, f.voter, f.bucket.Index, big.NewInt(1), staking.FrozenSelfStake{},
		)
		require.Error(t, err)
		var receiptErr staking.ReceiptError
		require.ErrorAs(t, err, &receiptErr)
		require.Equal(t, uint64(iotextypes.ReceiptStatus_ErrCandidateConflict), receiptErr.ReceiptStatus())
		require.True(t, compoundErrorIsChainDetermined(err))
	})

	t.Run("base view read failure halts", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)
		failing := &failingReadViewStateManager{
			StateManager: f.sm,
			err:          errors.New("view read failed"),
		}

		err := sp.AddDepositForCompound(
			f.ctx, failing, f.voter, f.bucket.Index, big.NewInt(1), staking.FrozenSelfStake{},
		)
		require.ErrorContains(t, err, "view read failed")
		require.False(t, compoundErrorIsChainDetermined(err))
	})

	t.Run("bucket write failure halts", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)
		failing := &failingPutStateManager{
			StateManager: f.sm,
			namespace:    state.StakingNamespace,
			err:          errors.New("bucket write failed"),
		}

		err := sp.AddDepositForCompound(
			f.ctx, failing, f.voter, f.bucket.Index, big.NewInt(1), staking.FrozenSelfStake{},
		)
		require.ErrorContains(t, err, "bucket write failed")
		require.False(t, compoundErrorIsChainDetermined(err))
	})

	t.Run("candidate write failure halts", func(t *testing.T) {
		f := newCompoundFixture(t)
		sp := staking.FindProtocol(protocol.MustGetRegistry(f.ctx))
		require.NotNil(t, sp)
		failing := &failingCandidatePutStateManager{
			StateManager: f.sm,
			err:          errors.New("candidate write failed"),
		}

		err := sp.AddDepositForCompound(
			f.ctx, failing, f.voter, f.bucket.Index, big.NewInt(1), staking.FrozenSelfStake{},
		)
		require.ErrorContains(t, err, "candidate write failed")
		require.False(t, compoundErrorIsChainDetermined(err))
	})
}

type failingReadViewStateManager struct {
	protocol.StateManager
	err error
}

func (f *failingReadViewStateManager) ReadView(string) (protocol.View, error) {
	return nil, f.err
}

type failingCandidatePutStateManager struct {
	protocol.StateManager
	err error
}

func (f *failingCandidatePutStateManager) PutState(
	s interface{}, opts ...protocol.StateOption,
) (uint64, error) {
	if _, ok := s.(*staking.Candidate); ok {
		return 0, f.err
	}
	return f.StateManager.PutState(s, opts...)
}

// TestVoterChunkErrorSettleabilityDefaultsToHalt pins the inversion that keeps
// the scan path non-degradable: a chunk error is settleable only when it was
// explicitly built as such.
//
// This is deliberately not a sentinel test. ErrNotSupported exists twice with
// identical message text (state/factory and db) and both are reachable from the
// drain's scan path, so any detector keyed on one of them would silently pass
// the other through as settleable. Opting in explicitly is what makes that
// impossible to get wrong by omission.
func TestVoterChunkErrorSettleabilityDefaultsToHalt(t *testing.T) {
	r := require.New(t)

	marked := settleableVoterChunkError("rewarding: voter chunk dispatched without a live cursor")
	r.True(voterChunkErrorIsSettleable(marked))
	r.True(voterChunkErrorIsSettleable(errors.Wrap(marked, "wrapped by a caller")),
		"wrapping must not lose the marking")

	r.False(voterChunkErrorIsSettleable(nil))
	r.False(voterChunkErrorIsSettleable(errors.New("rewarding: scan voter shard 7")))
	r.False(voterChunkErrorIsSettleable(errors.New("not supported")),
		"a capability error must never be settled into a Failure receipt")
	r.False(voterChunkErrorIsSettleable(errors.Wrap(state.ErrStateNotExist, "read")))
}

// ---------------------------------------------------------- R5 end-to-end --

// compoundFixture plants one delegate and one voter with an eligible
// auto-deposit bucket, and returns everything a routing test needs to drive
// payVoterCombined against real staking state.
type compoundFixture struct {
	ctx      context.Context
	sm       protocol.StateManager
	p        *Protocol
	delegate address.Address
	voter    address.Address
	bucket   *staking.VoteBucket
	routing  voterRouting
}

func newCompoundFixture(t *testing.T) compoundFixture {
	t.Helper()
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
	voter := staking.TestOnlyPerfBenchVoterAddress(0)

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

	bridge, err := autodeposit.New(identityset.Address(0).String())
	r.NoError(err)
	p.autoDepositBridge = bridge
	p.autoDepositBucketReaderFactory = func(autodeposit.SlotReader) autodeposit.BucketReader {
		return &registeredBucketReader{voter: voter, bucketID: compoundBucket.Index}
	}
	routing, err := p.resolveVoterRouting(ctx, sm)
	r.NoError(err)

	return compoundFixture{
		ctx: ctx, sm: sm, p: p,
		delegate: delegates[0], voter: voter, bucket: compoundBucket, routing: routing,
	}
}

// selfStakeMismatchShares builds a share set whose frozen work item claims the
// voter's ordinary bucket was the delegate's self-stake bucket at the era
// freeze. It was not, so AddDepositForCompound's frozen-vs-live guard fires
// with ErrCompoundSelfStakeRoleChanged -- row 10 of the classification table,
// and the cheapest chain-determined rejection to provoke without corrupting
// state, because the guard sits before the first mutation.
func selfStakeMismatchShares(
	delegate address.Address,
	bucketID uint64,
	amount *big.Int,
) (voterShareSet, voterShareInputs) {
	work := epochDrainDelegateWork{
		CandidateIdentifier: delegate.Bytes(),
		VoterAmountFrozen:   new(big.Int).Set(amount),
		SelfStakeBucketIdx:  bucketID,
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
			delegates:    delegates,
			byCandidate:  delegateWorkIndex(delegates),
			freezeHeight: iip59FixtureFreezeHeight,
			distributed:  []*big.Int{new(big.Int)},
		}
}

// TestPayVoterCombinedDegradesChainDeterminedCompoundFailure is the R5 happy
// case: a compound rejected on grounds every node can see must still pay the
// voter, by credit, so the drain cursor advances past them.
//
// Before this, any AddDepositForCompound error propagated out of the chunk and
// the block failed. One voter whose bucket had been unstaked since the freeze
// was enough to wedge the era's drain permanently: every retry replayed the
// same deterministic rejection.
func TestPayVoterCombinedDegradesChainDeterminedCompoundFailure(t *testing.T) {
	r := require.New(t)
	f := newCompoundFixture(t)
	testdb.AllowRevert(f.sm.(*mock_chainmanager.MockStateManager))

	amount := big.NewInt(777)
	stakedBefore := new(big.Int).Set(f.bucket.StakedAmount)
	shares, in := selfStakeMismatchShares(f.delegate, f.bucket.Index, amount)

	payout, err := f.p.payVoterCombined(f.ctx, f.sm, f.routing, in, f.voter, shares, &iip59RouteDurations{})
	r.NoError(err, "a chain-determined compound rejection must not fail the block")
	r.False(payout.compounded, "the payout must have been rerouted to a credit")
	r.Equal(f.voter.String(), payout.recipient.String())
	r.Zero(payout.amount.Cmp(amount))

	// The voter is paid, in full, at their reward destination.
	acct, err := accountutil.LoadAccount(f.sm, f.voter)
	r.NoError(err)
	r.Zero(acct.Balance.Cmp(amount), "the degraded voter must still be paid the whole share")

	// And a credit -- unlike a compound -- is an outflow from the rewarding
	// fund, so it must produce a transaction log.
	txLog := voterTransactionLog(payout)
	r.NotNil(txLog, "a degraded payout is a real transfer and must be logged as one")
	r.Equal(f.voter.String(), txLog.Recipient)

	// The rejection fired before the first mutation, so the bucket is untouched
	// and the share was not paid twice.
	updatedCSR, err := staking.ConstructBaseView(f.sm)
	r.NoError(err)
	updated, err := updatedCSR.NativeBucket(f.bucket.Index)
	r.NoError(err)
	r.Zero(updated.StakedAmount.Cmp(stakedBefore),
		"a degraded compound must leave the bucket exactly as it found it")
}

// failingPutStateManager makes every write to one namespace fail, which is the
// shape of an infrastructure fault: node-local, invisible to other validators,
// and therefore never safe to degrade.
type failingPutStateManager struct {
	protocol.StateManager
	namespace string
	err       error
}

func (f *failingPutStateManager) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err == nil && cfg.Namespace == f.namespace {
		return 0, f.err
	}
	return f.StateManager.PutState(s, opts...)
}

// TestPayVoterCombinedHaltsOnInfrastructureCompoundFailure is the other half of
// R5, and the one that protects consensus.
//
// A write failure inside AddDepositForCompound is node-local. If it degraded to
// a credit, a proposer with a failing disk would write a credit while every
// healthy validator wrote a compound: same block, two state roots. So it must
// propagate, and the voter must not be paid by the fallback path.
func TestPayVoterCombinedHaltsOnInfrastructureCompoundFailure(t *testing.T) {
	r := require.New(t)
	f := newCompoundFixture(t)
	testdb.AllowRevert(f.sm.(*mock_chainmanager.MockStateManager))

	amount := big.NewInt(777)
	shares, in := newRoutingShares(f.delegate, amount)
	failing := &failingPutStateManager{
		StateManager: f.sm,
		namespace:    state.StakingNamespace,
		err:          errors.New("disk on fire"),
	}

	_, err := f.p.payVoterCombined(f.ctx, failing, f.routing, in, f.voter, shares, &iip59RouteDurations{})
	r.Error(err, "an infrastructure failure must halt the chunk, never degrade")
	r.Contains(err.Error(), "disk on fire")

	acct, aErr := accountutil.LoadAccount(f.sm, f.voter)
	r.NoError(aErr)
	r.Zero(acct.Balance.Sign(),
		"a halting failure must not have paid the voter through the credit fallback")
}
