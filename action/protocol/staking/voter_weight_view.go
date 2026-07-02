// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"math/big"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
)

// VoterDeltaSink is the receiving end for (cand, voter, Δweight) tuples emitted
// by contract-staking event processing. Plumbing goes through context so we
// don't have to widen the ContractStakeView interface (which would break every
// mock and third-party impl). staking/protocol.go injects an adapter over
// viewData.voterWeights before calling contractsStake.Handle; stakingindex
// voteView pulls it back out and wraps its event handler.
type VoterDeltaSink interface {
	Apply(cand, voter address.Address, delta *big.Int)
}

type voterDeltaSinkKey struct{}

// WithVoterDeltaSink returns a child ctx carrying sink. Nil sink is a no-op.
func WithVoterDeltaSink(ctx context.Context, sink VoterDeltaSink) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, voterDeltaSinkKey{}, sink)
}

// VoterDeltaSinkFrom returns the sink from ctx if present, nil otherwise.
func VoterDeltaSinkFrom(ctx context.Context) VoterDeltaSink {
	if v := ctx.Value(voterDeltaSinkKey{}); v != nil {
		return v.(VoterDeltaSink)
	}
	return nil
}

// VoterWeightView maintains an incrementally-updated per-candidate map of
// voter → weight totals. Staking event handlers push (cand, voter, Δweight)
// deltas as buckets are created/unstaked/moved; PutPollResult reads back a
// sorted per-candidate slice to freeze the epoch's snapshot.
//
// The view is memory-only. On restart the base map is rebuilt by scanning
// current buckets — the frozen per-epoch snapshot blobs (_voterWeightSnap)
// are the durable half of the story, not this live view. This keeps the
// consensus path off the independent contract staking indexer db: contract
// staking event handling feeds this same view through the receipt-driven
// contractStakeView.Handle path, which is state.db-native.
//
// Lifecycle mirrors bucketPool / candidateCenter:
//   - CreateBaseView builds the initial base once, at protocol Start.
//   - Snapshot() Wrap()s the current view so an in-flight action can Revert.
//   - Fork() deep-clones for parallel working sets.
//   - Commit() folds pending change into the base at end-of-block.
type VoterWeightView interface {
	// Apply adds delta to (cand, voter). delta may be negative to model
	// weight loss (Unstake, ChangeCandidate off-ramp, contract Withdrawal).
	// A resulting zero entry is retained until Commit, when it is pruned.
	Apply(cand, voter address.Address, delta *big.Int)
	// Weights returns cand's voter list sorted by voter address bytes.
	// Zero-weight entries are filtered out. Returned slice is a fresh copy.
	Weights(cand address.Address) []VoterWeight
	// Wrap layers a change overlay on top of the receiver, without cloning
	// the base. Used at Snapshot time so Revert can throw the overlay away.
	Wrap() VoterWeightView
	// Fork deep-clones base and change into a fully independent view.
	Fork() VoterWeightView
	// Commit folds pending change into the base and prunes zeros. Returns
	// the resulting base (the receiver itself for a base view).
	Commit() VoterWeightView
	// IsDirty reports whether uncommitted changes exist.
	IsDirty() bool
}

// voterWeightBase is the concrete base implementation. Keys are raw
// address bytes cast to string (matches candidate_votes.go pattern).
type voterWeightBase struct {
	cands map[string]map[string]*big.Int
}

// voterWeightWrapper stacks a change overlay on top of a base view.
type voterWeightWrapper struct {
	base   VoterWeightView
	change *voterWeightBase
}

func newVoterWeightBase() *voterWeightBase {
	return &voterWeightBase{cands: map[string]map[string]*big.Int{}}
}

// NewVoterWeightView returns an empty base view.
func NewVoterWeightView() VoterWeightView {
	return newVoterWeightBase()
}

// --- voterWeightBase ---

func (v *voterWeightBase) Apply(cand, voter address.Address, delta *big.Int) {
	if cand == nil || voter == nil || delta == nil || delta.Sign() == 0 {
		return
	}
	ck := string(cand.Bytes())
	vk := string(voter.Bytes())
	inner, ok := v.cands[ck]
	if !ok {
		inner = map[string]*big.Int{}
		v.cands[ck] = inner
	}
	cur, ok := inner[vk]
	if !ok {
		cur = new(big.Int)
		inner[vk] = cur
	}
	cur.Add(cur, delta)
}

func (v *voterWeightBase) Weights(cand address.Address) []VoterWeight {
	if cand == nil {
		return nil
	}
	inner := v.cands[string(cand.Bytes())]
	if len(inner) == 0 {
		return nil
	}
	out := make([]VoterWeight, 0, len(inner))
	for vk, w := range inner {
		if w == nil || w.Sign() == 0 {
			continue
		}
		addr, err := address.FromBytes([]byte(vk))
		if err != nil {
			// Not reachable: keys come from address.Bytes() (always 20 bytes).
			continue
		}
		out = append(out, VoterWeight{Voter: addr, Weight: new(big.Int).Set(w)})
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Voter.Bytes(), out[j].Voter.Bytes()) < 0
	})
	return out
}

func (v *voterWeightBase) Wrap() VoterWeightView {
	return &voterWeightWrapper{base: v, change: newVoterWeightBase()}
}

func (v *voterWeightBase) Fork() VoterWeightView {
	clone := newVoterWeightBase()
	for ck, inner := range v.cands {
		if len(inner) == 0 {
			continue
		}
		dst := make(map[string]*big.Int, len(inner))
		for vk, w := range inner {
			if w == nil {
				continue
			}
			dst[vk] = new(big.Int).Set(w)
		}
		clone.cands[ck] = dst
	}
	return clone
}

func (v *voterWeightBase) Commit() VoterWeightView {
	// Prune zero entries and empty candidate buckets. Keeps the base map
	// bounded over time as voters churn.
	for ck, inner := range v.cands {
		for vk, w := range inner {
			if w == nil || w.Sign() == 0 {
				delete(inner, vk)
			}
		}
		if len(inner) == 0 {
			delete(v.cands, ck)
		}
	}
	return v
}

func (v *voterWeightBase) IsDirty() bool {
	// Base mutates in place; per-snapshot dirtiness is tracked by wrappers.
	return false
}

// --- voterWeightWrapper ---

func (w *voterWeightWrapper) Apply(cand, voter address.Address, delta *big.Int) {
	if delta == nil || delta.Sign() == 0 {
		return
	}
	w.change.Apply(cand, voter, delta)
}

func (w *voterWeightWrapper) Weights(cand address.Address) []VoterWeight {
	if cand == nil {
		return nil
	}
	// base is already sorted; merge change entries in via a map, then sort.
	sum := map[string]*big.Int{}
	for _, e := range w.base.Weights(cand) {
		sum[string(e.Voter.Bytes())] = new(big.Int).Set(e.Weight)
	}
	if inner := w.change.cands[string(cand.Bytes())]; len(inner) > 0 {
		for vk, d := range inner {
			if d == nil || d.Sign() == 0 {
				continue
			}
			cur, ok := sum[vk]
			if !ok {
				cur = new(big.Int)
				sum[vk] = cur
			}
			cur.Add(cur, d)
		}
	}
	out := make([]VoterWeight, 0, len(sum))
	for vk, s := range sum {
		if s.Sign() == 0 {
			continue
		}
		addr, err := address.FromBytes([]byte(vk))
		if err != nil {
			continue
		}
		out = append(out, VoterWeight{Voter: addr, Weight: s})
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].Voter.Bytes(), out[j].Voter.Bytes()) < 0
	})
	return out
}

func (w *voterWeightWrapper) Wrap() VoterWeightView {
	return &voterWeightWrapper{base: w, change: newVoterWeightBase()}
}

func (w *voterWeightWrapper) Fork() VoterWeightView {
	forked := &voterWeightWrapper{
		base:   w.base.Fork(),
		change: newVoterWeightBase(),
	}
	for ck, inner := range w.change.cands {
		if len(inner) == 0 {
			continue
		}
		dst := make(map[string]*big.Int, len(inner))
		for vk, d := range inner {
			if d == nil {
				continue
			}
			dst[vk] = new(big.Int).Set(d)
		}
		forked.change.cands[ck] = dst
	}
	return forked
}

func (w *voterWeightWrapper) Commit() VoterWeightView {
	// Fold change into base via the base's Apply — this preserves the
	// zero-collapse semantics uniformly.
	for ck, inner := range w.change.cands {
		candAddr, err := address.FromBytes([]byte(ck))
		if err != nil {
			continue
		}
		for vk, d := range inner {
			if d == nil || d.Sign() == 0 {
				continue
			}
			voterAddr, err := address.FromBytes([]byte(vk))
			if err != nil {
				continue
			}
			w.base.Apply(candAddr, voterAddr, d)
		}
	}
	w.change = newVoterWeightBase()
	return w.base.Commit()
}

func (w *voterWeightWrapper) IsDirty() bool {
	for _, inner := range w.change.cands {
		for _, d := range inner {
			if d != nil && d.Sign() != 0 {
				return true
			}
		}
	}
	return w.base.IsDirty()
}

// buildVoterWeightBaseFromState is the one-shot initial build invoked at
// protocol Start. It scans every registered candidate's native buckets from
// state.db and — for each contract staking indexer — enumerates the
// candidate's contract buckets, aggregates by voter address using the
// standard CalculateVoteWeight formula, and populates a fresh base view.
//
// Exclusions match the eventual reward-time semantics:
//   - Self-stake bucket (ContractAddress == "" && Index == cand.SelfStakeBucketIdx)
//   - Unstaked buckets (isUnstaked)
//   - Buckets with nil owner / nil amount (defensive)
//
// This runs once at Start; the ongoing view state is then maintained
// incrementally by handler + contract-staking-event Apply calls. Reading
// from the contract staking indexer db here is safe because Start is not a
// consensus mutation path — both nodes at the same height with the same
// (CheckIndexer-verified) indexer state produce the same base map.
func buildVoterWeightBaseFromState(
	csr CandidateStateReader,
	candidates CandidateList,
	indexers []ContractStakingIndexer,
	height uint64,
	weightConsts genesis.VoteWeightCalConsts,
) (VoterWeightView, error) {
	view := newVoterWeightBase()

	for _, cand := range candidates {
		if cand == nil {
			continue
		}
		candID := cand.GetIdentifier()
		if candID == nil {
			continue
		}

		// Native buckets.
		if err := aggregateNativeBucketsForCandidate(csr, cand, weightConsts, func(voter address.Address, w *big.Int) {
			view.Apply(candID, voter, w)
		}); err != nil {
			return nil, err
		}

		// Contract-staking buckets across all registered indexers.
		for _, idx := range indexers {
			if idx == nil {
				continue
			}
			bkts, err := idx.BucketsByCandidate(candID, height)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to read contract buckets from %s for candidate %s",
					idx.ContractAddress().String(), candID.String())
			}
			for _, bkt := range bkts {
				if bkt == nil || bkt.isUnstaked() {
					continue
				}
				if bkt.Owner == nil || bkt.StakedAmount == nil {
					continue
				}
				w := CalculateVoteWeight(weightConsts, bkt, false)
				if w == nil || w.Sign() == 0 {
					continue
				}
				view.Apply(candID, bkt.Owner, w)
			}
		}
	}
	return view, nil
}

// aggregateNativeBucketsForCandidate walks the native bucket index for cand
// and invokes emit for each valid voter contribution (self-stake and
// unstaked buckets excluded). Errors from state.ErrStateNotExist (no
// buckets tied to this cand) are swallowed; other errors propagate.
func aggregateNativeBucketsForCandidate(
	csr CandidateStateReader,
	cand *Candidate,
	weightConsts genesis.VoteWeightCalConsts,
	emit func(voter address.Address, weight *big.Int),
) error {
	nativeIdx, _, err := csr.NativeBucketIndicesByCandidate(cand.GetIdentifier())
	switch errors.Cause(err) {
	case nil:
	case state.ErrStateNotExist:
		return nil
	default:
		return errors.Wrapf(err, "failed to read native bucket indices for %s", cand.GetIdentifier().String())
	}
	if nativeIdx == nil {
		return nil
	}
	for _, i := range *nativeIdx {
		bkt, err := csr.NativeBucket(i)
		if errors.Cause(err) == state.ErrStateNotExist || errors.Cause(err) == ErrWithdrawnBucket {
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to read native bucket %d for candidate %s", i, cand.GetIdentifier().String())
		}
		if bkt == nil || bkt.isUnstaked() {
			continue
		}
		if bkt.ContractAddress == "" && bkt.Index == cand.SelfStakeBucketIdx {
			continue
		}
		if bkt.Owner == nil || bkt.StakedAmount == nil {
			continue
		}
		w := CalculateVoteWeight(weightConsts, bkt, false)
		if w == nil || w.Sign() == 0 {
			continue
		}
		emit(bkt.Owner, w)
	}
	return nil
}
