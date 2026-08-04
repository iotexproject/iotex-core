// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// TestVoterWeightInvariant pins the load-bearing assumption of the IIP-59 era
// redesign: for every candidate in the active set,
//
//	Candidate.Votes == Σ over voters of FrozenVoterWeight(cand, voter, ...)
//
// The right-hand side is not a model of the drain, it *is* the drain's
// arithmetic: computeVoterShares divides a delegate's frozen pool by
// candidate.Votes (frozen as CandidatePollSnapshot.TotalWeight) and multiplies
// by exactly this FrozenVoterWeight. So this compares the path-dependent
// running accumulator every staking handler maintains against the stateless
// recompute that will divide by it. If they can drift, every share in the era
// is wrong by the drift, and a drift upwards is money the era never set aside.
//
// This replaces an earlier version that compared candidate.Votes against a
// second incrementally-maintained table (the retired VoterWeightView). That
// test could only catch a handler updating one accumulator and not the other;
// this one also catches a handler whose accumulator diverges from the buckets
// themselves, which is the failure the redesign is actually exposed to. It is
// therefore strictly stronger than the test it replaces.
//
// The test drives real actions through Protocol.Handle rather than a model, so
// it exercises the actual choke points (addCandidateVotes / subCandidateVotes)
// and the real bucket writes the recompute reads back.
//
// Four preconditions, all inherited from the version this replaces and all
// still load-bearing:
//
//	V1. height >= XinguBlockHeight. Before Xingu, contract-staking weight is
//	    added at read time (StoreVoteOfNFTBucketIntoView) instead of living in
//	    candidate.Votes, so the two sides are not even measuring the same
//	    quantity. newVWInvariantEnv pulls every fork gate to height 1.
//	V2. No VoteReviser height at or after Xingu. A revise rebuilds
//	    candidate.Votes wholesale, which would move the denominator of an era
//	    already being drained. Pinned by the calculateVoteWeight exemption in
//	    TestCandidateVoteMutationsUseChokePoint.
//	V3. No handler subtracts more from candidate.Votes than the voter's own
//	    contribution. SubVote refuses to go negative, but a partial over-subtract
//	    is silent; this test is what makes it visible.
//	V4. Every vote mutation goes through addCandidateVotes / subCandidateVotes.
//	    Pinned structurally by voter_weight_chokepoint_test.go.
//
// The equality is asserted only for candidates in the *active set*, because
// only active candidates reach a poll result and therefore a frozen snapshot.
// isActiveCandidate uses the refined isSelfStakeBucket predicate while the
// recompute uses the stateless `bkt.Index == selfStakeBucketIdx`; a candidate
// on which those two disagree leaves the active set, which is what bounds the
// disagreement. See TestLapsedEndorsementDivergesSelfStakePredicates for the
// construction of that case, and the payout clamp in
// rewarding/voter_allocation.go for what happens if it is reached anyway.

// vwAction mirrors the unexported action.actionPayload interface so this test
// can drive heterogeneous staking actions through one helper.
type vwAction interface {
	IntrinsicGas() (uint64, error)
	SanityCheck() error
	FillAction(*iotextypes.ActionCore)
}

// vwInvariantGasLimit is a block/action gas limit large enough for every action
// this test submits.
const vwInvariantGasLimit = uint64(10_000_000)

// vwInvariantEnv is a working staking protocol driven at post-IIP-59 heights.
type vwInvariantEnv struct {
	t      *testing.T
	sm     protocol.StateManager
	p      *Protocol
	g      genesis.Genesis
	height uint64
	nonce  map[string]uint64
	now    time.Time
	// sawNonZero guards against the invariant passing vacuously because every
	// candidate happened to have zero votes and no frozen buckets.
	sawNonZero bool
}

// activateAllForks pulls every *BlockHeight fork gate in the genesis config to
// height 1, so a test running at heights 1..N sees a single coherent feature
// set instead of a mix of pre- and post-fork behaviour.
func activateAllForks(g genesis.Genesis) genesis.Genesis {
	v := reflect.ValueOf(&g.Blockchain).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if strings.HasSuffix(f.Name, "BlockHeight") && v.Field(i).Kind() == reflect.Uint64 {
			v.Field(i).SetUint(1)
		}
	}
	return g
}

func newVWInvariantEnv(t *testing.T) *vwInvariantEnv {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	r.NoError(err)

	// Run in the post-IIP-59 regime: that is the only regime in which the
	// invariant is load-bearing, and it is also the only one in which
	// contract-staking weight lands in cand.Votes rather than being added at
	// read time (StoreVoteOfNFTBucketIntoView = !IsXingu). Every fork is pulled
	// to height 1 so the feature set is self-consistent -- enabling Xingu while
	// leaving Fairbank at its mainnet height produces a regime that never
	// existed on chain.
	g := activateAllForks(genesis.TestDefault())

	p, err := NewProtocol(HelperCtx{
		DepositGas:    depositGas,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight: g.Staking.VoteWeightCalConsts,
		},
	}, nil, nil, nil, nil)
	r.NoError(err)

	env := &vwInvariantEnv{
		t:      t,
		sm:     sm,
		p:      p,
		g:      g,
		height: 1,
		nonce:  make(map[string]uint64),
		now:    time.Now(),
	}

	ctx := env.blockCtx(context.Background())
	v, err := p.Start(ctx, sm)
	r.NoError(err)
	vd, ok := v.(*viewData)
	r.True(ok)
	r.NoError(sm.WriteView(_protocolID, vd))
	return env
}

func (e *vwInvariantEnv) blockCtx(base context.Context) context.Context {
	ctx := protocol.WithBlockCtx(base, protocol.BlockCtx{
		BlockHeight:    e.height,
		BlockTimeStamp: e.now,
		GasLimit:       vwInvariantGasLimit,
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		Tip: protocol.TipInfo{Height: e.height - 1},
	})
	ctx = genesis.WithGenesisContext(ctx, e.g)
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

// fund gives an address enough balance to pay for whatever it is about to do.
func (e *vwInvariantEnv) fund(addr address.Address, iotx int64) {
	require.NoError(e.t, setupAccount(e.sm, addr, iotx))
}

// do submits one action from caller and asserts the receipt succeeded.
func (e *vwInvariantEnv) do(caller address.Address, act vwAction) {
	e.t.Helper()
	r := require.New(e.t)
	e.nonce[caller.String()]++
	nonce := e.nonce[caller.String()]
	elp := (&action.EnvelopeBuilder{}).SetNonce(nonce).
		SetGasLimit(vwInvariantGasLimit).SetGasPrice(testGasPrice).SetAction(act).Build()
	intrinsic, err := elp.IntrinsicGas()
	r.NoError(err)
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller:       caller,
		GasPrice:     testGasPrice,
		IntrinsicGas: intrinsic,
		Nonce:        nonce,
	})
	ctx = e.blockCtx(ctx)
	receipt, err := e.p.Handle(ctx, elp, e.sm)
	r.NoError(err)
	r.NotNil(receipt)
	r.EqualValuesf(1, receipt.Status, "action %T failed with receipt status %d", act, receipt.Status)
	e.height++
	e.now = e.now.Add(time.Minute)
}

// candCenter returns the candidate center the handlers just mutated. It comes
// from the same shared viewData that Protocol.Handle writes through, so this
// observes exactly the accumulator a freeze would capture as TotalWeight.
func (e *vwInvariantEnv) candCenter() (*CandidateCenter, CandidateStateManager) {
	r := require.New(e.t)
	csm, err := NewCandidateStateManager(e.sm)
	r.NoError(err)
	return csm.DirtyView().candCenter, csm
}

// frozenSums opens an era copy-on-write window at the current height, walks the
// whole voter key space exactly as the drain does, and returns
// Σ_voters FrozenVoterWeight(cand, voter, ...) keyed by candidate identifier.
//
// The window is opened and sealed around the walk rather than held open across
// the whole test, because a window that stayed open would make every later
// action a post-freeze write: the copy-on-write layer would keep resolving
// buckets to their value at the first checkpoint and the recompute would stop
// tracking the accumulator it is supposed to be compared against. Opening at
// the current height means "frozen" and "live" coincide, which is exactly the
// instant an era boundary captures TotalWeight at.
func (e *vwInvariantEnv) frozenSums(ctx context.Context) map[string]*big.Int {
	e.t.Helper()
	r := require.New(e.t)

	r.NoError(TestOnlyBeginEraCOWWindow(ctx, e.sm, e.height))
	defer func() { r.NoError(SealEraCOWWindow(ctx, e.sm)) }()
	window, err := eracow.LoadWindow(e.sm)
	r.NoError(err)
	r.True(window.Open(), "era window must be open for the recompute to resolve")

	// The nft event handler weighs contract buckets at csm.SR().Height(), so
	// the recompute has to use the same clock or the two sides of the equality
	// are evaluated at different heights. In production this is the delegate's
	// own FreezeHeight on both sides.
	evalHeight, err := e.sm.Height()
	r.NoError(err)

	_, csm := e.candCenter()
	sums := make(map[string]*big.Int)
	for shard := 0; shard < AddressShards; shard++ {
		voters, err := FrozenShardVoters(e.sm, window, byte(shard), nil)
		r.NoError(err)
		for _, voter := range voters {
			candidates, err := FrozenVoterCandidates(e.sm, window, voter)
			r.NoError(err)
			for _, candID := range candidates {
				selfStakeIdx := uint64(candidateNoSelfStakeBucketIndex)
				if cand := csm.GetByIdentifier(candID); cand != nil {
					selfStakeIdx = cand.SelfStakeBucketIdx
				}
				w, err := FrozenVoterWeight(
					e.sm, window, e.p, candID, voter, selfStakeIdx, evalHeight,
				)
				r.NoError(err)
				key := string(candID.Bytes())
				if sums[key] == nil {
					sums[key] = new(big.Int)
				}
				sums[key].Add(sums[key], w)
			}
		}
	}
	return sums
}

// checkInvariant asserts candidate.Votes == Σ_voters FrozenVoterWeight(...) for
// every candidate in the active set.
func (e *vwInvariantEnv) checkInvariant(step string) {
	e.t.Helper()
	r := require.New(e.t)
	ctx := e.blockCtx(context.Background())
	sums := e.frozenSums(ctx)
	cc, csm := e.candCenter()

	for _, cand := range cc.All() {
		active, err := e.p.isActiveCandidate(ctx, csm, cand)
		r.NoError(err)
		if !active {
			// Not in the active set means not in the poll result, so not in a
			// frozen snapshot, so never a denominator. Asserting here would be
			// asserting about a candidate no era can pay.
			continue
		}
		sum := sums[string(cand.GetIdentifier().Bytes())]
		if sum == nil {
			sum = new(big.Int)
		}
		if sum.Sign() > 0 {
			e.sawNonZero = true
		}
		r.Truef(cand.Votes.Cmp(sum) == 0,
			"after %q: active candidate %s (%s) Votes=%s but Σ FrozenVoterWeight=%s (drift %s)",
			step, cand.Name, cand.GetIdentifier().String(),
			cand.Votes.String(), sum.String(),
			new(big.Int).Sub(cand.Votes, sum).String())
	}
}

func (e *vwInvariantEnv) bucketIndexOf(owner address.Address) uint64 {
	r := require.New(e.t)
	csr := newCandidateStateReader(e.sm)
	indices, _, err := csr.NativeBucketIndicesByVoter(owner)
	r.NoError(err)
	r.NotEmpty(indices)
	return (*indices)[len(*indices)-1]
}

// registerCandidate registers a candidate with a self-stake bucket through the
// real handler, so its initial Votes and its view seed both come from
// production code.
func (e *vwInvariantEnv) registerCandidate(name string, ownerIdx, operatorIdx int) *Candidate {
	r := require.New(e.t)
	owner := identityset.Address(ownerIdx)
	e.fund(owner, 2_000_000)
	act, err := action.NewCandidateRegister(
		name,
		identityset.Address(operatorIdx).String(),
		owner.String(),
		owner.String(),
		unit.ConvertIotxToRau(1_200_000).String(),
		7,
		true,
		nil,
	)
	r.NoError(err)
	e.do(owner, act)
	cc, _ := e.candCenter()
	cand := cc.GetByOwner(owner)
	r.NotNil(cand)
	return cand
}

func TestVoterWeightInvariant(t *testing.T) {
	r := require.New(t)
	e := newVWInvariantEnv(t)

	e.checkInvariant("empty state")

	// --- candidate registration (creates a self-stake bucket) ---
	candA := e.registerCandidate("canda", 1, 7)
	e.checkInvariant("register candA")
	candB := e.registerCandidate("candb", 2, 8)
	e.checkInvariant("register candB")

	voter1 := identityset.Address(11)
	voter2 := identityset.Address(12)
	voter3 := identityset.Address(13)
	for _, v := range []address.Address{voter1, voter2, voter3} {
		e.fund(v, 1_000_000)
	}

	// --- create stake ---
	cs1, err := action.NewCreateStake("canda", unit.ConvertIotxToRau(100).String(), 7, true, nil)
	r.NoError(err)
	e.do(voter1, cs1)
	e.checkInvariant("createStake voter1 -> candA")
	b1 := e.bucketIndexOf(voter1)

	cs2, err := action.NewCreateStake("canda", unit.ConvertIotxToRau(200).String(), 14, false, nil)
	r.NoError(err)
	e.do(voter2, cs2)
	e.checkInvariant("createStake voter2 -> candA")
	b2 := e.bucketIndexOf(voter2)

	cs3, err := action.NewCreateStake("candb", unit.ConvertIotxToRau(300).String(), 21, true, nil)
	r.NoError(err)
	e.do(voter3, cs3)
	e.checkInvariant("createStake voter3 -> candB")
	b3 := e.bucketIndexOf(voter3)

	// --- deposit to stake (weight grows in place) ---
	dep, err := action.NewDepositToStake(b1, unit.ConvertIotxToRau(50).String(), nil)
	r.NoError(err)
	e.do(voter1, dep)
	e.checkInvariant("depositToStake b1")

	// --- restake (duration / autostake change re-weights the bucket) ---
	e.do(voter2, action.NewRestake(b2, 30, true, nil))
	e.checkInvariant("restake b2")

	// --- change candidate (weight moves between candidates, same voter) ---
	e.do(voter2, action.NewChangeCandidate("candb", b2, nil))
	e.checkInvariant("changeCandidate b2 candA -> candB")

	// --- transfer stake (weight moves between voters, same candidate) ---
	newOwner := identityset.Address(14)
	e.fund(newOwner, 1_000)
	ts, err := action.NewTransferStake(newOwner.String(), b3, nil)
	r.NoError(err)
	e.do(voter3, ts)
	e.checkInvariant("transferStake b3 voter3 -> newOwner")

	// --- contract-staking (NFT) bucket lifecycle ---
	// Reached directly through nftEventHandler: HandleReceipt needs a wired
	// contract-staking indexer, which a unit test cannot supply, but the
	// handler is the code that actually moves cand.Votes and the view.
	e.checkContractStakingBuckets(candB)

	// --- turn off auto-stake so the bucket can mature and be unstaked ---
	e.do(voter1, action.NewRestake(b1, 7, false, nil))
	e.checkInvariant("restake b1 autoStake=false")

	// --- unstake (weight leaves the candidate entirely) ---
	e.now = e.now.Add(8 * 24 * time.Hour)
	e.do(voter1, action.NewUnstake(b1, nil))
	e.checkInvariant("unstake b1")

	// --- withdraw after the waiting period ---
	e.now = e.now.Add(e.g.WithdrawWaitingPeriod + time.Hour)
	e.do(voter1, action.NewWithdrawStake(b1, nil))
	e.checkInvariant("withdrawStake b1")

	// --- endorsement + activate: moves the 1.06x self-stake bonus onto a
	// bucket that is not the candidate's own, which re-weights it on both
	// sides. This is the exact quantity the redesign reads out of the frozen
	// per-candidate record, so a drift here would be silently baked into every
	// share of the era.
	endorser := identityset.Address(16)
	e.fund(endorser, 2_000_000)
	csE, err := action.NewCreateStake("candb", unit.ConvertIotxToRau(1_200_000).String(), 91, true, nil)
	r.NoError(err)
	e.do(endorser, csE)
	e.checkInvariant("createStake endorser -> candB")
	bE := e.bucketIndexOf(endorser)

	endorse, err := action.NewCandidateEndorsement(bE, action.CandidateEndorsementOpEndorse)
	r.NoError(err)
	e.do(endorser, endorse)
	e.checkInvariant("candidateEndorsement endorse bE")

	e.do(candB.Owner, action.NewCandidateActivate(bE))
	e.checkInvariant("candidateActivate bE (self-stake bonus moves)")

	// --- candidate ownership transfer (may clear the self-stake bonus) ---
	newCandOwner := identityset.Address(15)
	e.fund(newCandOwner, 1_000)
	cto, err := action.NewCandidateTransferOwnership(newCandOwner.String(), nil)
	r.NoError(err)
	e.do(candA.Owner, cto)
	e.checkInvariant("candidateTransferOwnership candA")

	r.True(e.sawNonZero, "invariant never observed a non-zero recomputed weight: the test is vacuous")
}

// checkContractStakingBuckets exercises the NFT/contract-staking event handler,
// which is the post-Xingu path that folds contract bucket weight into
// cand.Votes, and whose owner-index writes are what let the drain find the
// bucket again from the voter side.
func (e *vwInvariantEnv) checkContractStakingBuckets(cand *Candidate) {
	r := require.New(e.t)
	ctx := e.blockCtx(context.Background())
	if protocol.MustGetFeatureCtx(ctx).StoreVoteOfNFTBucketIntoView {
		// Pre-Xingu the contract bucket weight is kept out of cand.Votes
		// entirely, so the invariant is not expected to hold for contract
		// buckets at those heights. This is precondition V1.
		e.t.Log("skipping contract-staking leg: StoreVoteOfNFTBucketIntoView is on at this height")
		return
	}
	// The block context, not context.Background(): the handler passes it down
	// to the contract-staking state manager, which needs the IIP-59 gate open
	// to maintain the LSD owner index. Without the index the drain cannot
	// enumerate this bucket from its voter and the recompute would silently
	// come up short.
	handler, err := newNFTBucketEventHandler(ctx, e.sm, e.p.calculateContractBucketVoteWeight)
	r.NoError(err)

	contract := identityset.Address(30)
	csVoter := identityset.Address(31)
	bkt := &contractstaking.Bucket{
		Candidate:        cand.GetIdentifier(),
		Owner:            csVoter,
		StakedAmount:     unit.ConvertIotxToRau(1000),
		StakedDuration:   uint64((91 * 24 * time.Hour).Seconds()),
		CreatedAt:        uint64(e.now.Unix()),
		UnlockedAt:       MaxDurationNumber, // auto-stake
		UnstakedAt:       MaxDurationNumber, // not unstaked
		IsTimestampBased: true,
	}
	r.NoError(handler.PutBucket(contract, 1, bkt))
	// The era window freezes each contract's bucket high-water mark as the
	// bound on which ids could have existed at H. A real indexer maintains it;
	// a unit test that plants a bucket without it would have the recompute
	// reject the bucket as post-freeze and see a false drift.
	r.NoError(contractstaking.NewContractStakingStateManager(e.sm).UpdateNumOfBuckets(contract, 1))
	e.checkInvariant("contract-staking PutBucket")

	r.NoError(handler.DeleteBucket(contract, 1))
	e.checkInvariant("contract-staking DeleteBucket")
}
