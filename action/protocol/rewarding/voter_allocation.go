// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
)

// voterDelegateShare is one voter's claim on one delegate's frozen voter pool.
type voterDelegateShare struct {
	delegateIndex int
	candidate     address.Address
	weight        *big.Int
	share         *big.Int
	clamped       bool
}

// voterShareSet is the whole answer for one voter: a share per delegate the
// voter had a frozen bucket with, and their sum, which is what actually moves.
type voterShareSet struct {
	shares []voterDelegateShare
	total  *big.Int
}

// voterShareInputs is the state a share computation reads. Bundled so the drain
// and the read-only status query cannot pass different things by accident.
type voterShareInputs struct {
	window       eracow.Window
	staking      *staking.Protocol
	delegates    []epochDrainDelegateWork
	byCandidate  map[string]int
	freezeHeight uint64
	distributed  []*big.Int
}

// computeVoterShares is the single implementation of the per-voter share rule.
//
// Two callers need it: the drain, which pays, and the read-only status query,
// which reports what a voter is owed. Keeping one implementation behind both is
// what makes TestVoterRewardAmountMatchesAllocation a statement about the code
// rather than a coincidence.
//
// The rule is: for each delegate the voter has a frozen bucket with, recompute
// the voter's weight toward that delegate as of the era freeze height, take
// floor(pool * weight / totalWeight), clamp it to what is left of the pool, and
// sum. Delegates outside the frozen work list contribute nothing.
//
// evalHeight is the plan's FreezeHeight and never the height of the block this
// runs in. A contract bucket that is not timestamp-based has its
// remaining duration measured against a block height, so the same bucket is
// worth different amounts in chunk 1 and chunk 5 of the same drain if the
// current height leaks in here.
func computeVoterShares(
	sr protocol.StateReader,
	in voterShareInputs,
	voter address.Address,
) (voterShareSet, error) {
	out := voterShareSet{total: new(big.Int)}
	candidates, err := staking.FrozenVoterCandidates(sr, in.window, voter)
	if err != nil {
		return out, err
	}
	for _, candidate := range candidates {
		i, ok := in.byCandidate[string(candidate.Bytes())]
		if !ok || i >= len(in.delegates) {
			continue
		}
		work := in.delegates[i]
		pool := safeBig(work.VoterAmountFrozen)
		totalWeight := safeBig(work.TotalWeight)
		if pool.Sign() <= 0 || totalWeight.Sign() <= 0 {
			continue
		}
		if in.freezeHeight == 0 {
			// The recompute is height-sensitive and there is no height to hand
			// it. Refuse rather than substitute the current block's.
			return out, errors.Errorf(
				"rewarding: delegate %s work item has no freeze height", candidate.String(),
			)
		}
		weight, err := staking.FrozenVoterWeight(
			sr, in.window, in.staking, candidate, voter,
			work.SelfStakeBucketIdx, in.freezeHeight,
		)
		if err != nil {
			return out, errors.Wrapf(err,
				"rewarding: recompute weight of voter %s for delegate %s",
				voter.String(), candidate.String())
		}
		if weight.Sign() <= 0 {
			continue
		}
		share := new(big.Int).Mul(pool, weight)
		share.Div(share, totalWeight)

		// ---- the payout clamp -------------------------------------------
		//
		// Floor division guarantees that the shares sum to at most the pool
		// only if the per-voter weights sum to at most TotalWeight. Here they
		// need not: TotalWeight is the frozen value of candidate.Votes, a
		// running accumulator that every staking handler adds to and subtracts
		// from as buckets change, while the weight above is a stateless
		// recompute from the buckets themselves. A path-dependent total and a
		// stateless recompute can disagree, and when the recomputed weights sum
		// to more than the frozen total the naive shares sum to more than the
		// pool -- the drain would pay out money the era never set aside.
		//
		// One known source of disagreement is deliberately left in place. The
		// recompute decides the self-stake bonus with `bkt.Index ==
		// selfStakeBucketIdx`, while every candidate.Votes mutator uses the
		// refined isSelfStakeBucket predicate. The two diverge because
		// endorsement expiry is passive -- an endorsement lapses at a height
		// with no transaction to observe it -- while SelfStakeBucketIdx is
		// cleared only on an explicit revoke. No stateless recompute can
		// reproduce a path-dependent accumulator, so this is not fixable by
		// changing either predicate, and changing one to match the other would
		// only move the disagreement. It is bounded in practice because
		// isActiveCandidate uses the refined predicate, so a candidate in this
		// state leaves the active set and stops accruing rewards.
		//
		// The clamp turns every such mismatch into under-payment. A numerator
		// that is too large stops at the pool boundary; a numerator that is too
		// small leaves a residual in the pending pool for a future era. Both are
		// safe; over-payment is not. It also bounds
		// violations of the candidate.Votes invariant preconditions V2 and V3:
		// however far the accumulator has drifted, the pool is still the
		// ceiling.
		remaining := new(big.Int).Sub(pool, safeBig(in.distributed[i]))
		clamped := false
		if remaining.Sign() < 0 {
			remaining.SetInt64(0)
		}
		if share.Cmp(remaining) > 0 {
			share = remaining
			clamped = true
		}
		if share.Sign() <= 0 {
			continue
		}
		out.shares = append(out.shares, voterDelegateShare{
			delegateIndex: i,
			candidate:     candidate,
			weight:        weight,
			share:         share,
			clamped:       clamped,
		})
		out.total.Add(out.total, share)
	}
	return out, nil
}

// delegateWorkIndex maps candidate identifier bytes to a position in the frozen
// work list. Built once per call; only ever read by key, never iterated, so it
// introduces no map-ordering nondeterminism.
func delegateWorkIndex(delegates []epochDrainDelegateWork) map[string]int {
	byCandidate := make(map[string]int, len(delegates))
	for i := range delegates {
		byCandidate[string(delegates[i].CandidateIdentifier)] = i
	}
	return byCandidate
}

// distributedVector returns the per-delegate running totals, padded to the
// delegate count so callers can index it without a bounds check.
func distributedVector(c *epochDrainCursor) []*big.Int {
	out := make([]*big.Int, len(c.Delegates))
	for i := range out {
		if i < len(c.Distributed) && c.Distributed[i] != nil {
			out[i] = new(big.Int).Set(c.Distributed[i])
			continue
		}
		out[i] = new(big.Int)
	}
	return out
}

// candidateBytesEqual reports whether a work item names the given candidate.
func candidateBytesEqual(work epochDrainDelegateWork, candidate address.Address) bool {
	return candidate != nil && bytes.Equal(work.CandidateIdentifier, candidate.Bytes())
}
