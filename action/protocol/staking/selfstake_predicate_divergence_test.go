// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// TestLapsedEndorsementDivergesSelfStakePredicates documents the one place the
// two self-stake predicates IIP-59 depends on can give different answers about
// the same bucket, and pins where that is and is not reachable.
//
// There are two predicates, deliberately not unified:
//
//   - the stateless one, `bkt.Index == selfStakeBucketIdx`, used by
//     FrozenVoterWeight. It is the numerator side of every voter payout, and it
//     has to be stateless: the drain spans blocks and recomputes weights long
//     after the boundary, so it may only read what the era froze.
//   - the refined one, isSelfStakeBucket, used by every candidate.Votes mutator
//     and by isActiveCandidate. It consults the endorsement record, so it can
//     answer "no" for a bucket that is still registered as a candidate's
//     self-stake bucket.
//
// They diverge when an endorsement lapses *passively*. Endorsement.LegacyStatus
// is a bare `height >= ExpireHeight` comparison: there is no transaction at that
// height and nothing observes it, while Candidate.SelfStakeBucketIdx is cleared
// only by an explicit revoke (clearCandidateSelfStake, whose sole call site is
// handleCandidateEndorsement, with no per-block sweep anywhere). So from the
// expiry height onwards the refined predicate says "not self-stake" and the
// stateless one still says "self-stake", and the gap between the two weights is
// the 1.06x self-stake bonus.
//
// Neither predicate is wrong and neither can be changed to match the other: no
// stateless recompute can reproduce a path-dependent accumulator, and making
// candidate.Votes stateless would mean rebuilding it from every bucket on every
// mutation. What contains the disagreement is (a) isActiveCandidate using the
// refined predicate, so a candidate in this state leaves the active set and
// therefore never reaches a poll result or a frozen snapshot, and (b) the payout
// clamp in rewarding/voter_allocation.go for the residue that historical state
// can still carry in. The clamp half is pinned by
// TestLapsedSelfStakeBonusCannotOverpayDelegatePool in the rewarding package.
//
// The second subtest is the finding this test exists to record: the passive
// expiry is a *legacy-mode* behaviour. EnforceLegacyEndorsement is
// !IsUpernavik(height) and Upernavik precedes Xingu, so at every height IIP-59
// can run at, EndorsementStateReader.Status uses Endorsement.Status, which
// returns UnEndorsing rather than EndorseExpired and never lets an endorsement
// lapse on its own. The divergence therefore cannot be *created* post-Xingu; it
// can only be inherited, as a skew already baked into candidate.Votes by a
// bucket mutation that happened while the legacy rules were live. Anything that
// gives Status a passive-expiry branch would make it creatable again, and the
// second subtest is what fails when that happens.
func TestLapsedEndorsementDivergesSelfStakePredicates(t *testing.T) {
	const (
		evalHeight    = uint64(100)
		expireHeight  = uint64(50)
		upernavikLate = uint64(1_000)
	)

	// legacyGenesis puts `evalHeight` inside [Tsunami, Upernavik): the
	// endorsement feature exists, and its status is still read with
	// LegacyStatus. Only the two endorsement gates are moved; every other fork
	// stays where TestDefault put it, because no other gate participates in
	// either predicate.
	legacyGenesis := func() genesis.Genesis {
		g := genesis.TestDefault()
		g.TsunamiBlockHeight = 1
		g.UpernavikBlockHeight = upernavikLate
		return g
	}

	newEnv := func(t *testing.T, g genesis.Genesis, height uint64) (
		context.Context, *Protocol, CandidateStateManager, *Candidate, *VoteBucket,
	) {
		r := require.New(t)
		ctrl := gomock.NewController(t)
		sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
		// The endorsement reader compares ExpireHeight against the state
		// manager's height, not the block height, so it is the one that has to
		// be past the expiry for the lapse to be observable.
		sm.EXPECT().Height().Return(evalHeight, nil).AnyTimes()

		owner := identityset.Address(1)
		endorser := identityset.Address(2)
		// 1.2M IOTX for 91 days with auto-stake on: the two conditions
		// CalculateVoteWeight requires before it will apply the self-stake
		// bonus at all. Without them the two predicates would still disagree
		// but the disagreement would be worth nothing, and the test would pass
		// vacuously.
		selfStake, ok := new(big.Int).SetString("1200000000000000000000000", 10)
		r.True(ok)
		bucket := NewVoteBucket(owner, endorser, selfStake, 91, time.Now(), true)
		bucket.Index = 0

		cand := &Candidate{
			Owner:              owner,
			Operator:           identityset.Address(3),
			Reward:             identityset.Address(4),
			Name:               "lapsed",
			Votes:              new(big.Int),
			SelfStakeBucketIdx: bucket.Index,
			SelfStake:          new(big.Int).Set(selfStake),
		}
		cc, err := NewCandidateCenter(CandidateList{cand})
		r.NoError(err)
		r.NoError(sm.WriteView(_protocolID, &viewData{
			candCenter: cc,
			bucketPool: &BucketPool{
				enableSMStorage: true,
				total:           &totalAmount{amount: big.NewInt(0)},
			},
		}))
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		_, err = csm.putBucket(bucket)
		r.NoError(err)

		// The endorsement that lapses. Nothing revokes it; it simply stops
		// being valid at expireHeight, which is the whole point.
		r.NoError(NewEndorsementStateManager(sm).Put(bucket.Index, &Endorsement{
			ExpireHeight: expireHeight,
		}))

		p := &Protocol{
			config: Configuration{
				VoteWeightCalConsts:    g.VoteWeightCalConsts,
				RegistrationConsts:     RegistrationConsts{MinSelfStake: big.NewInt(1)},
				MinSelfStakeToBeActive: big.NewInt(1),
			},
		}
		ctx := genesis.WithGenesisContext(context.Background(), g)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		ctx = protocol.WithFeatureCtx(ctx)
		return ctx, p, csm, cand, bucket
	}

	t.Run("legacy endorsement rules: the predicates disagree", func(t *testing.T) {
		r := require.New(t)
		ctx, p, csm, cand, bucket := newEnv(t, legacyGenesis(), evalHeight)
		fCtx := protocol.MustGetFeatureCtx(ctx)
		r.True(fCtx.EnforceLegacyEndorsement, "fixture must be pre-Upernavik")
		r.False(fCtx.DisableDelegateEndorsement, "fixture must be post-Tsunami")

		// (a) The two predicates genuinely disagree about this bucket.
		stateless := bucket.Index == cand.SelfStakeBucketIdx
		refined, err := isSelfStakeBucket(fCtx, csm, bucket)
		r.NoError(err)
		r.True(stateless, "FrozenVoterWeight still sees a self-stake bucket")
		r.False(refined, "every candidate.Votes mutator no longer sees one")

		// The endorsement really did lapse without a transaction: nothing was
		// deleted, SelfStakeBucketIdx still names the bucket, and only the
		// height moved.
		endorsement, err := NewEndorsementStateReader(csm.SR()).Get(bucket.Index)
		r.NoError(err)
		r.Equal(EndorseExpired, endorsement.LegacyStatus(evalHeight))
		r.Equal(bucket.Index, cand.SelfStakeBucketIdx)

		// The disagreement is worth money: it is exactly the self-stake bonus,
		// which is the amount by which a frozen numerator can exceed what the
		// accumulator would record for the same bucket.
		withBonus := p.calculateVoteWeight(bucket, stateless)
		withoutBonus := p.calculateVoteWeight(bucket, refined)
		r.Positive(withBonus.Cmp(withoutBonus),
			"the predicates must disagree by a non-zero weight or this proves nothing")

		// (b) What bounds it: the candidate drops out of the active set, so it
		// reaches neither the poll result nor a frozen snapshot.
		active, err := p.isActiveCandidate(ctx, csm, cand)
		r.NoError(err)
		r.False(active, "a candidate whose endorsement lapsed must leave the active set")
	})

	t.Run("post-Upernavik rules: passive expiry no longer exists", func(t *testing.T) {
		r := require.New(t)
		g := legacyGenesis()
		ctx, p, csm, cand, bucket := newEnv(t, g, upernavikLate+1)
		fCtx := protocol.MustGetFeatureCtx(ctx)
		r.False(fCtx.EnforceLegacyEndorsement, "fixture must be post-Upernavik")

		// Same state, same expired ExpireHeight, opposite answer: Status maps
		// an elapsed ExpireHeight to UnEndorsing, and isSelfStakeBucket only
		// rejects EndorseExpired. So the refined predicate agrees with the
		// stateless one again.
		endorsement, err := NewEndorsementStateReader(csm.SR()).Get(bucket.Index)
		r.NoError(err)
		r.Equal(UnEndorsing, endorsement.Status(evalHeight))

		refined, err := isSelfStakeBucket(fCtx, csm, bucket)
		r.NoError(err)
		r.True(refined,
			"post-Upernavik an endorsement no longer lapses on its own; if this "+
				"fails, passive expiry is reachable at IIP-59 heights again and "+
				"the payout clamp is the only thing left containing it")

		active, err := p.isActiveCandidate(ctx, csm, cand)
		r.NoError(err)
		r.True(active)
	})
}
