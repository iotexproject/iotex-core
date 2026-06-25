// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// _voterWeightsKey is the single state-trie key under StakingNamespace where
// IIP-59's view digest lives. The 1-byte namespace tag _voterWeights = 5 is
// reserved in protocol.go.
var _voterWeightsKey = []byte{_voterWeights}

// voterWeightDigest is the serialized form of VoterWeightView.Hash() — one
// 32-byte value per chain, rewritten only when the view is dirty at block
// commit time. Deserialize requires the buffer to be exactly 32 bytes so a
// corrupted record fails loudly rather than silently producing a zero hash.
type voterWeightDigest struct {
	Hash hash.Hash256
}

// Serialize implements state.Serializer.
func (d *voterWeightDigest) Serialize() ([]byte, error) {
	out := make([]byte, len(hash.Hash256{}))
	copy(out, d.Hash[:])
	return out, nil
}

// Deserialize implements state.Deserializer.
func (d *voterWeightDigest) Deserialize(buf []byte) error {
	if len(buf) != len(hash.Hash256{}) {
		return errors.Errorf("voter weight digest must be %d bytes, got %d", len(hash.Hash256{}), len(buf))
	}
	copy(d.Hash[:], buf)
	return nil
}

// readVoterWeightDigest returns the persisted view digest, if present.
func readVoterWeightDigest(sm protocol.StateReader) (hash.Hash256, error) {
	d := &voterWeightDigest{}
	if _, err := sm.State(d,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(_voterWeightsKey),
	); err != nil {
		return hash.ZeroHash256, err
	}
	return d.Hash, nil
}

// VoterWeightView tracks per-candidate per-voter weighted votes for IIP-59
// protocol-native voter reward distribution.
//
// Lifecycle mirrors ContractStakeView (Wrap/Fork/Commit/IsDirty) so the IIP-59
// view plugs into the existing staking viewData snapshot/revert machinery
// without paying for a full data clone on every snapshot:
//
//   - Wrap()        returns a thin overlay sharing the same base — used by
//                   viewData.Snapshot. Cheap: no data copy.
//   - Fork()        returns an overlay with commit-in-clone semantics — used
//                   by viewData.Fork. The base is shared until the fork
//                   commits; only then is the base cloned, so parents that
//                   never see a fork commit pay nothing.
//   - Commit(sm)    flattens this layer's accumulated deltas into the
//                   underlying base, persists the new digest if dirty, and
//                   returns the new (collapsed) view.
//   - IsDirty()     true if any change has accumulated since the last Commit.
//
// Determinism: per-candidate voter slices are kept sorted by voter address so
// VoterWeightsByCandidate iterates them in a stable order — distributeVoterReward
// must never iterate a Go map to compute receipt-log order or state writes
// (see #4811 review #2 for the bug class).
type VoterWeightView interface {
	// Apply adjusts the weight that voter contributes to candidate by delta.
	// Positive delta = new stake / restake / change-candidate-in;
	// negative delta = unstake / change-candidate-out. Aggregates per
	// (cand, voter) so distribution doesn't pay per-bucket rounding loss.
	Apply(candID hash.Hash160, voter address.Address, delta *big.Int)
	// VoterWeightsByCandidate returns the per-voter weight contributions for
	// the given candidate, sorted by voter address. Returns nil if the
	// candidate has no active voters.
	VoterWeightsByCandidate(candID hash.Hash160) []voterWeight
	// Hash returns a deterministic 32-byte digest of the materialized view —
	// identical across nodes for the same logical state, regardless of the
	// underlying overlay topology.
	Hash() hash.Hash256
	// Wrap returns an overlay used by viewData.Snapshot. Changes made through
	// the overlay flow into the base on Commit; discarding the overlay (via
	// viewData.Revert) drops the changes.
	Wrap() VoterWeightView
	// Fork returns an overlay used by viewData.Fork. Differs from Wrap only
	// at Commit time: the base is cloned before deltas merge in, so the
	// pre-fork view is preserved for any other holders.
	Fork() VoterWeightView
	// Commit flattens this layer's deltas into the base, persists the new
	// digest when sm is non-nil and the view is dirty, and returns the
	// collapsed view that the caller should install in its viewData.
	Commit(sm protocol.StateManager) (VoterWeightView, error)
	// IsDirty reports whether any Apply has run since the last Commit.
	IsDirty() bool
}

// voterWeight is a single (voter, weighted-votes) pair belonging to some
// candidate. Multiple buckets from the same voter to the same delegate are
// aggregated into a single entry, so the protocol distributes per voter, not
// per bucket — this avoids per-bucket rounding loss.
type voterWeight struct {
	voter  address.Address
	weight *big.Int
}

// candidateVoterEntry holds the per-(candidate, voter) weighted votes for one
// candidate, kept sorted by voter address. The index map gives O(log n)
// lookups.
type candidateVoterEntry struct {
	sorted []voterWeight
	index  map[hash.Hash160]int
}

// voterWeightBase is the concrete in-memory state. It holds the full sorted
// per-candidate per-voter weight table and serves as the terminal layer for
// the Wrap/Fork chain.
type voterWeightBase struct {
	byCandidate map[hash.Hash160]*candidateVoterEntry
	dirty       bool
}

// voterWeightWrap is the lazy overlay used by viewData.Snapshot. It
// accumulates deltas in `change` and reads through to `base`. Commit replays
// the change deltas into the base directly (parent is mutated).
type voterWeightWrap struct {
	base   VoterWeightView
	change *voterWeightChange
}

// voterWeightFork is the commit-in-clone overlay used by viewData.Fork. The
// base is cloned only when Commit actually flushes deltas, so workingsets
// that fork-and-discard pay nothing.
type voterWeightFork struct {
	*voterWeightWrap
}

// voterWeightChange is a delta accumulator used by overlays. Unlike
// voterWeightBase it stores raw deltas (any sign), keyed by candidate and
// voter address; merging into a base reproduces the net effect of all the
// Apply calls that funneled through the overlay.
type voterWeightChange struct {
	byCandidate map[hash.Hash160]map[hash.Hash160]*voterDelta
}

type voterDelta struct {
	voter address.Address
	delta *big.Int // any sign; zero means no net change but the entry stays so it's flushed on Commit
}

// NewVoterWeightView returns an empty base view.
func NewVoterWeightView() VoterWeightView {
	return newVoterWeightBase()
}

func newVoterWeightBase() *voterWeightBase {
	return &voterWeightBase{
		byCandidate: make(map[hash.Hash160]*candidateVoterEntry),
	}
}

func newVoterWeightChange() *voterWeightChange {
	return &voterWeightChange{
		byCandidate: make(map[hash.Hash160]map[hash.Hash160]*voterDelta),
	}
}

// -------- voterWeightBase --------

func (b *voterWeightBase) Apply(candID hash.Hash160, voter address.Address, delta *big.Int) {
	if delta == nil || delta.Sign() == 0 {
		return
	}
	b.dirty = true
	entry, ok := b.byCandidate[candID]
	if !ok {
		// Negative delta against an empty candidate is a programming error
		// upstream — the handler computed a withdrawal but the view does
		// not know about the voter. We silently treat as no-op rather than
		// crash; the view-hash check at restart catches any real divergence
		// (see ensureVoterWeightView) and surfaces it loudly.
		if delta.Sign() < 0 {
			return
		}
		entry = &candidateVoterEntry{index: make(map[hash.Hash160]int)}
		b.byCandidate[candID] = entry
	}

	voterID := hash.BytesToHash160(voter.Bytes())
	if slot, ok := entry.index[voterID]; ok {
		newWeight := new(big.Int).Add(entry.sorted[slot].weight, delta)
		if newWeight.Sign() <= 0 {
			entry.removeAt(slot)
			if len(entry.sorted) == 0 {
				delete(b.byCandidate, candID)
			}
			return
		}
		entry.sorted[slot].weight = newWeight
		return
	}
	if delta.Sign() < 0 {
		return
	}
	entry.insertSorted(voter, voterID, new(big.Int).Set(delta))
}

func (b *voterWeightBase) VoterWeightsByCandidate(candID hash.Hash160) []voterWeight {
	entry, ok := b.byCandidate[candID]
	if !ok || len(entry.sorted) == 0 {
		return nil
	}
	out := make([]voterWeight, len(entry.sorted))
	for i, vw := range entry.sorted {
		out[i] = voterWeight{voter: vw.voter, weight: new(big.Int).Set(vw.weight)}
	}
	return out
}

func (b *voterWeightBase) Hash() hash.Hash256 {
	if len(b.byCandidate) == 0 {
		return hash.ZeroHash256
	}
	candIDs := make([]hash.Hash160, 0, len(b.byCandidate))
	for id := range b.byCandidate {
		candIDs = append(candIDs, id)
	}
	sort.Slice(candIDs, func(i, j int) bool {
		return bytes.Compare(candIDs[i][:], candIDs[j][:]) < 0
	})

	var buf bytes.Buffer
	scratch := make([]byte, 8)
	for _, candID := range candIDs {
		buf.Write(candID[:])
		entry := b.byCandidate[candID]
		binary.BigEndian.PutUint64(scratch, uint64(len(entry.sorted)))
		buf.Write(scratch)
		for _, vw := range entry.sorted {
			buf.Write(vw.voter.Bytes())
			wbytes := vw.weight.Bytes()
			binary.BigEndian.PutUint32(scratch[:4], uint32(len(wbytes)))
			buf.Write(scratch[:4])
			buf.Write(wbytes)
		}
	}
	return hash.Hash256b(buf.Bytes())
}

func (b *voterWeightBase) Wrap() VoterWeightView {
	return &voterWeightWrap{base: b, change: newVoterWeightChange()}
}

func (b *voterWeightBase) Fork() VoterWeightView {
	return &voterWeightFork{
		voterWeightWrap: &voterWeightWrap{base: b, change: newVoterWeightChange()},
	}
}

func (b *voterWeightBase) Commit(sm protocol.StateManager) (VoterWeightView, error) {
	if !b.dirty {
		return b, nil
	}
	if sm != nil {
		if _, err := sm.PutState(
			&voterWeightDigest{Hash: b.Hash()},
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(_voterWeightsKey),
		); err != nil {
			return b, errors.Wrap(err, "failed to persist voter weights digest")
		}
	}
	b.dirty = false
	return b, nil
}

func (b *voterWeightBase) IsDirty() bool {
	return b != nil && b.dirty
}

// clone returns a deep copy of the base, used by Fork's commit-in-clone path
// (and by Hash-via-flatten on an overlay). Package-private — the standard
// lifecycle goes through Wrap/Fork.
func (b *voterWeightBase) clone() *voterWeightBase {
	if b == nil {
		return nil
	}
	out := newVoterWeightBase()
	out.dirty = b.dirty
	for candID, entry := range b.byCandidate {
		c := &candidateVoterEntry{
			sorted: make([]voterWeight, len(entry.sorted)),
			index:  make(map[hash.Hash160]int, len(entry.index)),
		}
		for i, vw := range entry.sorted {
			c.sorted[i] = voterWeight{voter: vw.voter, weight: new(big.Int).Set(vw.weight)}
		}
		for k, slot := range entry.index {
			c.index[k] = slot
		}
		out.byCandidate[candID] = c
	}
	return out
}

// -------- voterWeightWrap (Snapshot overlay) --------

func (w *voterWeightWrap) Apply(candID hash.Hash160, voter address.Address, delta *big.Int) {
	if delta == nil || delta.Sign() == 0 {
		return
	}
	w.change.add(candID, voter, delta)
}

func (w *voterWeightWrap) VoterWeightsByCandidate(candID hash.Hash160) []voterWeight {
	return mergedVoters(w.base, w.change, candID)
}

func (w *voterWeightWrap) Hash() hash.Hash256 {
	return flatten(w).Hash()
}

func (w *voterWeightWrap) Wrap() VoterWeightView {
	return &voterWeightWrap{base: w, change: newVoterWeightChange()}
}

func (w *voterWeightWrap) Fork() VoterWeightView {
	return &voterWeightFork{
		voterWeightWrap: &voterWeightWrap{base: w, change: newVoterWeightChange()},
	}
}

func (w *voterWeightWrap) Commit(sm protocol.StateManager) (VoterWeightView, error) {
	w.flushIntoBase(w.base)
	return w.base.Commit(sm)
}

func (w *voterWeightWrap) IsDirty() bool {
	if w == nil {
		return false
	}
	return !w.change.empty() || w.base.IsDirty()
}

// flushIntoBase replays all accumulated deltas into the target. The caller
// supplies the target so commit-in-clone (Fork) can redirect into a cloned
// base instead of the shared one.
func (w *voterWeightWrap) flushIntoBase(target VoterWeightView) {
	w.change.forEach(func(candID hash.Hash160, voter address.Address, delta *big.Int) {
		target.Apply(candID, voter, delta)
	})
	w.change = newVoterWeightChange()
}

// -------- voterWeightFork (Fork overlay, commit-in-clone) --------

func (f *voterWeightFork) Commit(sm protocol.StateManager) (VoterWeightView, error) {
	// Detach from the shared base before flushing — the parent must not
	// observe any of the fork's deltas. Then proceed as a normal wrap commit.
	baseClone := flatten(f.base)
	f.base = baseClone
	f.flushIntoBase(baseClone)
	return baseClone.Commit(sm)
}

func (f *voterWeightFork) Wrap() VoterWeightView {
	return &voterWeightWrap{base: f, change: newVoterWeightChange()}
}

func (f *voterWeightFork) Fork() VoterWeightView {
	return &voterWeightFork{
		voterWeightWrap: &voterWeightWrap{base: f, change: newVoterWeightChange()},
	}
}

// -------- voterWeightChange (delta accumulator) --------

func (c *voterWeightChange) add(candID hash.Hash160, voter address.Address, delta *big.Int) {
	voterID := hash.BytesToHash160(voter.Bytes())
	inner, ok := c.byCandidate[candID]
	if !ok {
		inner = make(map[hash.Hash160]*voterDelta)
		c.byCandidate[candID] = inner
	}
	if existing, ok := inner[voterID]; ok {
		existing.delta = new(big.Int).Add(existing.delta, delta)
		return
	}
	inner[voterID] = &voterDelta{voter: voter, delta: new(big.Int).Set(delta)}
}

func (c *voterWeightChange) empty() bool {
	return c == nil || len(c.byCandidate) == 0
}

// forEach calls fn for every accumulated delta. Iteration order is
// non-deterministic — only safe for operations that are commutative (like
// replaying into a base view).
func (c *voterWeightChange) forEach(fn func(candID hash.Hash160, voter address.Address, delta *big.Int)) {
	for candID, voters := range c.byCandidate {
		for _, vd := range voters {
			fn(candID, vd.voter, vd.delta)
		}
	}
}

// -------- shared helpers --------

// flatten returns a deep-copied base reflecting the materialized state of
// any layered view. The returned base is a fresh value, safe for the caller
// to mutate. Used by Fork's commit-in-clone and by Hash on overlays.
func flatten(v VoterWeightView) *voterWeightBase {
	switch x := v.(type) {
	case *voterWeightBase:
		return x.clone()
	case *voterWeightWrap:
		out := flatten(x.base)
		x.change.forEach(func(candID hash.Hash160, voter address.Address, delta *big.Int) {
			out.Apply(candID, voter, delta)
		})
		return out
	case *voterWeightFork:
		return flatten(x.voterWeightWrap)
	default:
		// Unknown impl — defensive return of an empty view. Tests that
		// extend the interface should add a case.
		return newVoterWeightBase()
	}
}

// mergedVoters resolves the sorted (voter, weight) list for one candidate
// across an overlay, combining the read-through base with the local delta
// accumulator. Result is deterministically sorted by voter address.
func mergedVoters(base VoterWeightView, change *voterWeightChange, candID hash.Hash160) []voterWeight {
	baseList := base.VoterWeightsByCandidate(candID)
	deltas := change.byCandidate[candID]
	if len(deltas) == 0 {
		return baseList
	}
	merged := make(map[hash.Hash160]voterWeight, len(baseList)+len(deltas))
	for _, vw := range baseList {
		vid := hash.BytesToHash160(vw.voter.Bytes())
		merged[vid] = voterWeight{voter: vw.voter, weight: new(big.Int).Set(vw.weight)}
	}
	for vid, vd := range deltas {
		if cur, ok := merged[vid]; ok {
			cur.weight = new(big.Int).Add(cur.weight, vd.delta)
			if cur.weight.Sign() <= 0 {
				delete(merged, vid)
			} else {
				merged[vid] = cur
			}
		} else if vd.delta.Sign() > 0 {
			merged[vid] = voterWeight{voter: vd.voter, weight: new(big.Int).Set(vd.delta)}
		}
	}
	if len(merged) == 0 {
		return nil
	}
	out := make([]voterWeight, 0, len(merged))
	for _, vw := range merged {
		out = append(out, vw)
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare(out[i].voter.Bytes(), out[j].voter.Bytes()) < 0
	})
	return out
}

// insertSorted inserts (voter, weight) keeping entry.sorted sorted by voter
// address. The entry's index map is rebuilt for affected slots.
func (e *candidateVoterEntry) insertSorted(voter address.Address, voterID hash.Hash160, weight *big.Int) {
	pos := sort.Search(len(e.sorted), func(i int) bool {
		thisID := hash.BytesToHash160(e.sorted[i].voter.Bytes())
		return bytes.Compare(thisID[:], voterID[:]) >= 0
	})
	e.sorted = append(e.sorted, voterWeight{})
	copy(e.sorted[pos+1:], e.sorted[pos:])
	e.sorted[pos] = voterWeight{voter: voter, weight: weight}
	for i := pos; i < len(e.sorted); i++ {
		id := hash.BytesToHash160(e.sorted[i].voter.Bytes())
		e.index[id] = i
	}
}

// removeAt removes the entry at slot, compacts the slice, and rebuilds the
// index map for slots shifted by the removal.
func (e *candidateVoterEntry) removeAt(slot int) {
	id := hash.BytesToHash160(e.sorted[slot].voter.Bytes())
	delete(e.index, id)
	e.sorted = append(e.sorted[:slot], e.sorted[slot+1:]...)
	for i := slot; i < len(e.sorted); i++ {
		shiftedID := hash.BytesToHash160(e.sorted[i].voter.Bytes())
		e.index[shiftedID] = i
	}
}

// -------- initial population + handler hook --------

// buildVoterWeightView constructs a fresh VoterWeightView from a snapshot of
// active native + contract-staking buckets. Used once per chain when the
// IIP-59 feature flag first activates (via CreateBaseView), and at restart to
// reconstruct the view and verify it against the persisted digest.
//
// candidateForBucket translates a bucket's candidate identifier to the
// candidate object; nil means "candidate not found, skip the bucket".
// Self-stake bonus is gated on b.ContractAddress == "" so contract buckets
// (which always have Index = 0) don't accidentally claim the bonus — PoC
// #4811 review finding #5.
func buildVoterWeightView(
	allBuckets []*VoteBucket,
	candidateForBucket func(*VoteBucket) *Candidate,
	consts genesis.VoteWeightCalConsts,
) VoterWeightView {
	v := newVoterWeightBase()
	for _, b := range allBuckets {
		if b == nil || b.isUnstaked() {
			continue
		}
		cand := candidateForBucket(b)
		if cand == nil {
			continue
		}
		isSelfStake := b.ContractAddress == "" && b.Index == cand.SelfStakeBucketIdx
		w := CalculateVoteWeight(consts, b, isSelfStake)
		if w.Sign() == 0 {
			continue
		}
		v.Apply(hash.BytesToHash160(cand.GetIdentifier().Bytes()), b.Owner, w)
	}
	// Initial build matches on-disk state at this height, so commit is a
	// no-op until the next mutation.
	v.dirty = false
	return v
}

// applyVoterWeightDelta is the single entry point every staking handler uses
// to keep the IIP-59 VoterWeightView in sync with on-chain bucket changes.
// No-op when the feature flag has not activated yet (view is nil) and when
// delta is zero, so callers can wire it next to any existing
// candidate.AddVote / candidate.SubVote site without first checking the flag.
//
// candIdentifier must be the candidate's identifier address (not operator)
// — same key the view uses internally. voter is the bucket owner.
func applyVoterWeightDelta(csm CandidateStateManager, candIdentifier address.Address, voter address.Address, delta *big.Int) {
	if delta == nil || delta.Sign() == 0 {
		return
	}
	view := csm.DirtyView()
	if view == nil || view.voterWeights == nil {
		return
	}
	view.voterWeights.Apply(hash.BytesToHash160(candIdentifier.Bytes()), voter, delta)
}
