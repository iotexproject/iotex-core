// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// _voterWeightKeyLen is the length of a per-(candidate, voter) state key: the
// 1-byte namespace tag reserved in protocol.go, followed by the 20-byte
// candidate identifier and the 20-byte voter address.
const _voterWeightKeyLen = 1 + 2*len(hash.Hash160{})

// voterWeightKey returns the state-trie key holding one voter's weight for one
// candidate. One key per pair, rather than one blob per candidate, is what lets
// a staking action rewrite only the pairs it touched: a delegate with 30k
// voters would otherwise pay to rewrite the entire set on every deposit.
func voterWeightKey(candID, voterID hash.Hash160) []byte {
	key := make([]byte, 0, _voterWeightKeyLen)
	key = append(key, _voterWeights)
	key = append(key, candID[:]...)
	key = append(key, voterID[:]...)
	return key
}

// parseVoterWeightKey reverses voterWeightKey. ok is false for any key in the
// staking namespace that is not a voter weight entry — buckets, candidate
// indices and endorsements all live in the same namespace, so a scan has to
// discriminate by key rather than by whether the value happens to deserialize.
func parseVoterWeightKey(key []byte) (candID, voterID hash.Hash160, ok bool) {
	if len(key) != _voterWeightKeyLen || key[0] != _voterWeights {
		return candID, voterID, false
	}
	copy(candID[:], key[1:1+len(candID)])
	copy(voterID[:], key[1+len(candID):])
	return candID, voterID, true
}

// voterWeightEntry is a single voter's aggregated weighted votes for a single
// candidate, held as first-class committed state.
//
// This is the whole point of the layout: the aggregate the reward distribution
// reads is the value the network agreed on, not a separately-derived quantity
// checked against a hash. There is nothing for a restart to disagree with.
type voterWeightEntry struct {
	Weight *big.Int
}

// Serialize implements state.Serializer. A stored weight is always strictly
// positive — the view removes an entry the moment it reaches zero — so writing
// anything else means the caller computed a weight it should have deleted.
func (e *voterWeightEntry) Serialize() ([]byte, error) {
	if e.Weight == nil || e.Weight.Sign() <= 0 {
		return nil, errors.Errorf("voter weight must be positive, got %v", e.Weight)
	}
	return e.Weight.Bytes(), nil
}

// Deserialize implements state.Deserializer.
func (e *voterWeightEntry) Deserialize(buf []byte) error {
	w := new(big.Int).SetBytes(buf)
	if w.Sign() <= 0 {
		return errors.Errorf("voter weight must be positive, got %s", w.String())
	}
	e.Weight = w
	return nil
}

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (e *voterWeightEntry) Encode() (systemcontracts.GenericValue, error) {
	data, err := e.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (e *voterWeightEntry) Decode(v systemcontracts.GenericValue) error {
	return e.Deserialize(v.PrimaryData)
}

// voterWeightPersistenceEnabled reports whether voter weight entries may be
// written to the state trie.
//
// Pre-activation they must stay out of it: nodes upgrade over days, and one
// that wrote these keys early would diverge from every node still on the old
// binary — a split at deployment time rather than at activation. The in-memory
// view is maintained on both sides of the fork, so only the write waits.
func voterWeightPersistenceEnabled(ctx context.Context) bool {
	fCtx, ok := protocol.GetFeatureCtx(ctx)
	return ok && !fCtx.NoVoterRewardDistribution
}

// readVoterWeightEntries loads every persisted (candidate, voter) weight into a
// fresh base view, and reports how many it found.
//
// The staking namespace holds buckets, candidate indices and endorsements as
// well, so entries are selected by key shape. A value that fails to deserialize
// under a key that IS ours is a real error; under any other key it is just
// another record's bytes and is skipped.
func readVoterWeightEntries(sr protocol.StateReader) (*voterWeightBase, int, error) {
	_, iter, err := sr.States(protocol.NamespaceOption(_stakingNameSpace))
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return newVoterWeightBase(), 0, nil
		}
		return nil, 0, errors.Wrap(err, "failed to scan staking namespace for voter weights")
	}

	v := newVoterWeightBase()
	count := 0
	for i := 0; i < iter.Size(); i++ {
		entry := &voterWeightEntry{}
		key, dErr := iter.Next(entry)
		candID, voterID, ok := parseVoterWeightKey(key)
		if !ok {
			continue
		}
		if dErr != nil {
			return nil, 0, errors.Wrapf(dErr, "failed to read voter weight entry %x", key)
		}
		voter, aErr := address.FromBytes(voterID[:])
		if aErr != nil {
			return nil, 0, errors.Wrapf(aErr, "invalid voter address in key %x", key)
		}
		v.Apply(candID, voter, entry.Weight)
		count++
	}
	// The view now matches on-disk state exactly, so nothing is pending.
	v.dirty = false
	v.touched = nil
	return v, count, nil
}

// loadVoterWeightView produces the view a node starts with.
//
// Once IIP-59 has activated, the per-(candidate, voter) entries in state are the
// single derivation of these weights: they are loaded, not recomputed, so a
// restart has nothing to compare and no way to disagree with the network. This
// is what removes the startup halt — the failure mode is now a wrong reward
// that every node computes identically and a later fix can correct, rather than
// a fleet that cannot boot.
//
// Before activation, and while the activation flush is still running, the
// persisted entries cover only part of the voter set, so the view is built from
// buckets exactly as it was before the weights became state. The seed cursor is
// what distinguishes the two cases — an entry count cannot, because a partial
// flush also has entries.
func loadVoterWeightView(
	sr protocol.StateReader,
	allBuckets []*VoteBucket,
	candidateForBucket func(*VoteBucket) *Candidate,
	consts genesis.VoteWeightCalConsts,
) (VoterWeightView, error) {
	seeded, err := voterWeightSeedingComplete(sr)
	if err != nil {
		return nil, err
	}
	if !seeded {
		return buildVoterWeightView(allBuckets, candidateForBucket, consts), nil
	}
	persisted, _, err := readVoterWeightEntries(sr)
	if err != nil {
		return nil, err
	}
	return persisted, nil
}

// VoterWeightView tracks per-candidate per-voter weighted votes for IIP-59
// protocol-native voter reward distribution.
//
// Wrap/Fork/Commit/IsDirty mirror ContractStakeView so this view plugs into the
// existing staking viewData snapshot/revert machinery. Overlays share a base
// and accumulate deltas, so no snapshot pays for a full data clone; see the
// individual methods for how each layer resolves at Commit.
//
// Per-candidate voter slices are kept sorted by voter address, because receipt
// log order and state write order are consensus-visible and must never come
// from a Go map iteration.
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
	// MarkForRewrite schedules (candID, voterID)'s current weight to be written
	// at the next Commit even though no delta changed it. Used only by the
	// activation-time seeding flush.
	MarkForRewrite(candID, voterID hash.Hash160)
	// SeedPairsAfter returns up to limit pairs in (candidate, voter) key order,
	// strictly after the given position; a nil position starts at the first
	// pair, and a negative limit returns all of them. Used only by the
	// activation-time seeding flush.
	//
	// Callers must invoke this before any action has run in the block — the
	// overlay layers answer from their base, which is exact only while no
	// deltas have accumulated. CreatePreStates satisfies this by construction.
	SeedPairsAfter(after *voterWeightRef, limit int) []voterWeightRef
	// Commit flattens this layer's deltas into the base, writes the entries
	// those deltas touched when sm is non-nil and IIP-59 has activated, and
	// returns the collapsed view that the caller should install in its
	// viewData.
	Commit(ctx context.Context, sm protocol.StateManager) (VoterWeightView, error)
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

// voterWeightWriter is the part of protocol.StateManager that persisting voter
// weights needs. Narrowing it keeps the write path honest about what it touches
// and lets it be exercised against a plain key/value stand-in.
type voterWeightWriter interface {
	PutState(interface{}, ...protocol.StateOption) (uint64, error)
	DelState(...protocol.StateOption) (uint64, error)
}

// voterWeightRef identifies one persisted entry: the (candidate, voter) pair
// whose weight lives under voterWeightKey(cand, voter).
type voterWeightRef struct {
	cand  hash.Hash160
	voter hash.Hash160
}

// voterWeightBase is the concrete in-memory state. It holds the full sorted
// per-candidate per-voter weight table and serves as the terminal layer for
// the Wrap/Fork chain.
//
// It is a write-through cache of the entries in state, not a second derivation
// of them: Commit writes exactly the pairs recorded in touched, and startup
// loads the result back. dirty tracks whether the view changed at all (it also
// flips for a mutation that turned out to be a no-op, which is what viewData
// callers have always observed); touched carries the keys those changes landed
// on, and is the only thing Commit needs to write.
type voterWeightBase struct {
	byCandidate map[hash.Hash160]*candidateVoterEntry
	dirty       bool
	touched     map[voterWeightRef]struct{}
}

// voterWeightWrap is the lazy overlay used by viewData.Snapshot. It
// accumulates deltas in `change` and reads through to `base`. Commit replays
// the change deltas into the base directly (parent is mutated).
type voterWeightWrap struct {
	base   VoterWeightView
	change *voterWeightChange
	// rewrite holds pairs the seeding flush scheduled through this layer. They
	// carry no delta — only the instruction to write the pair's current value —
	// so they travel separately from change and are replayed alongside it.
	rewrite map[voterWeightRef]struct{}
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

// touch records that (candID, voterID) must be rewritten at the next Commit.
// The map is allocated lazily so a view that is only read never pays for it.
func (b *voterWeightBase) touch(candID, voterID hash.Hash160) {
	if b.touched == nil {
		b.touched = make(map[voterWeightRef]struct{})
	}
	b.touched[voterWeightRef{cand: candID, voter: voterID}] = struct{}{}
}

// weightOf returns the current weight for a pair, or nil if the voter no longer
// contributes to that candidate — which is how Commit tells a write from a
// delete.
func (b *voterWeightBase) weightOf(ref voterWeightRef) *big.Int {
	entry, ok := b.byCandidate[ref.cand]
	if !ok {
		return nil
	}
	slot, ok := entry.index[ref.voter]
	if !ok {
		return nil
	}
	return entry.sorted[slot].weight
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
		// A withdrawal against a candidate the view has never seen. Staying a
		// no-op keeps the consensus result unchanged, but this can only happen
		// if a hook is missing or a delta is wrong upstream, so record it: this
		// is the one moment the bug is visible at its cause.
		if delta.Sign() < 0 {
			reportVoterWeightAnomaly(_vwAnomalyUnknownCandidate, candID, voter, delta)
			return
		}
		entry = &candidateVoterEntry{index: make(map[hash.Hash160]int)}
		b.byCandidate[candID] = entry
	}

	voterID := hash.BytesToHash160(voter.Bytes())
	b.touch(candID, voterID)
	if slot, ok := entry.index[voterID]; ok {
		newWeight := new(big.Int).Add(entry.sorted[slot].weight, delta)
		if newWeight.Sign() < 0 {
			// Overshoot: more weight was subtracted than was ever added, e.g. a
			// double-subtract, or an unstake/withdraw pair that both applied a
			// delta. Landing exactly on zero is the normal way a voter leaves;
			// going below it never is.
			reportVoterWeightAnomaly(_vwAnomalyUnderflow, candID, voter, delta)
		}
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
		reportVoterWeightAnomaly(_vwAnomalyUnknownVoter, candID, voter, delta)
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

func (b *voterWeightBase) MarkForRewrite(candID, voterID hash.Hash160) {
	if b.weightOf(voterWeightRef{cand: candID, voter: voterID}) == nil {
		// The pair left the view between being listed and being marked. Its
		// absolute value is already whatever the deltas made it, and a delete
		// for a key that was never written is a no-op, so there is nothing to
		// schedule.
		return
	}
	b.dirty = true
	b.touch(candID, voterID)
}

func (b *voterWeightBase) SeedPairsAfter(after *voterWeightRef, limit int) []voterWeightRef {
	if limit == 0 || len(b.byCandidate) == 0 {
		return nil
	}
	candIDs := make([]hash.Hash160, 0, len(b.byCandidate))
	for id := range b.byCandidate {
		candIDs = append(candIDs, id)
	}
	sort.Slice(candIDs, func(i, j int) bool {
		return bytes.Compare(candIDs[i][:], candIDs[j][:]) < 0
	})

	out := []voterWeightRef{}
	for _, candID := range candIDs {
		if after != nil && bytes.Compare(candID[:], after.cand[:]) < 0 {
			continue
		}
		entry := b.byCandidate[candID]
		start := 0
		if after != nil && candID == after.cand {
			// Voters within a candidate are already sorted, so resume just past
			// the cursor rather than rescanning from the front.
			start = sort.Search(len(entry.sorted), func(i int) bool {
				id := hash.BytesToHash160(entry.sorted[i].voter.Bytes())
				return bytes.Compare(id[:], after.voter[:]) > 0
			})
		}
		for i := start; i < len(entry.sorted); i++ {
			out = append(out, voterWeightRef{
				cand:  candID,
				voter: hash.BytesToHash160(entry.sorted[i].voter.Bytes()),
			})
			if limit > 0 && len(out) == limit {
				return out
			}
		}
	}
	return out
}

func (b *voterWeightBase) Commit(ctx context.Context, sm protocol.StateManager) (VoterWeightView, error) {
	if !b.dirty {
		return b, nil
	}
	if sm != nil && voterWeightPersistenceEnabled(ctx) {
		if err := b.persist(sm); err != nil {
			return b, err
		}
	}
	b.dirty = false
	b.touched = nil
	return b, nil
}

// persist writes the pairs mutated since the last Commit. Cost is proportional
// to what the block actually changed — typically one or two entries — not to
// the size of the view.
//
// Writes go out in key order. The trie root does not depend on it, but the
// Erigon dual-store and any write-ordered log do, and a deterministic order
// costs nothing here.
func (b *voterWeightBase) persist(sm voterWeightWriter) error {
	if len(b.touched) == 0 {
		return nil
	}
	refs := make([]voterWeightRef, 0, len(b.touched))
	for ref := range b.touched {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool { return refLess(refs[i], refs[j]) })

	for _, ref := range refs {
		key := voterWeightKey(ref.cand, ref.voter)
		weight := b.weightOf(ref)
		if weight == nil {
			// The voter's weight reached zero, so the pair leaves the state
			// rather than being stored as a zero. A key that was never written
			// (touched pre-activation, or touched and reverted) is not an error.
			if _, err := sm.DelState(
				protocol.NamespaceOption(_stakingNameSpace),
				protocol.KeyOption(key),
				protocol.ObjectOption(&voterWeightEntry{}),
			); err != nil && errors.Cause(err) != state.ErrStateNotExist {
				return errors.Wrapf(err, "failed to delete voter weight entry %x", key)
			}
			continue
		}
		if _, err := sm.PutState(
			&voterWeightEntry{Weight: weight},
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(key),
		); err != nil {
			return errors.Wrapf(err, "failed to persist voter weight entry %x", key)
		}
	}
	return nil
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
	// The clone inherits the pending writes: a fork that commits is responsible
	// for the parent's uncommitted pairs as well as its own, exactly as it
	// already inherits the parent's dirty flag.
	for ref := range b.touched {
		out.touch(ref.cand, ref.voter)
	}
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

func (w *voterWeightWrap) MarkForRewrite(candID, voterID hash.Hash160) {
	if w.rewrite == nil {
		w.rewrite = make(map[voterWeightRef]struct{})
	}
	w.rewrite[voterWeightRef{cand: candID, voter: voterID}] = struct{}{}
}

func (w *voterWeightWrap) SeedPairsAfter(after *voterWeightRef, limit int) []voterWeightRef {
	if !w.change.empty() {
		// Answering from the base would miss this layer's deltas. Seeding runs
		// in CreatePreStates, before any action, so the overlay is always empty
		// here; reaching this means it was called from somewhere else.
		reportVoterWeightAnomaly(_vwAnomalySeedOnDirtyOverlay, hash.Hash160{}, nil, nil)
	}
	return w.base.SeedPairsAfter(after, limit)
}

func (w *voterWeightWrap) Wrap() VoterWeightView {
	return &voterWeightWrap{base: w, change: newVoterWeightChange()}
}

func (w *voterWeightWrap) Fork() VoterWeightView {
	return &voterWeightFork{
		voterWeightWrap: &voterWeightWrap{base: w, change: newVoterWeightChange()},
	}
}

func (w *voterWeightWrap) Commit(ctx context.Context, sm protocol.StateManager) (VoterWeightView, error) {
	w.flushIntoBase(w.base)
	return w.base.Commit(ctx, sm)
}

func (w *voterWeightWrap) IsDirty() bool {
	if w == nil {
		return false
	}
	return !w.change.empty() || len(w.rewrite) > 0 || w.base.IsDirty()
}

// flushIntoBase replays all accumulated deltas into the target. The caller
// supplies the target so commit-in-clone (Fork) can redirect into a cloned
// base instead of the shared one.
func (w *voterWeightWrap) flushIntoBase(target VoterWeightView) {
	w.change.forEach(func(candID hash.Hash160, voter address.Address, delta *big.Int) {
		target.Apply(candID, voter, delta)
	})
	w.change = newVoterWeightChange()
	// Rewrites go in after the deltas so a pair that both moved and was
	// scheduled lands on its final value.
	for ref := range w.rewrite {
		target.MarkForRewrite(ref.cand, ref.voter)
	}
	w.rewrite = nil
}

// -------- voterWeightFork (Fork overlay, commit-in-clone) --------

func (f *voterWeightFork) Commit(ctx context.Context, sm protocol.StateManager) (VoterWeightView, error) {
	// Detach from the shared base before flushing — the parent must not
	// observe any of the fork's deltas. Then proceed as a normal wrap commit.
	baseClone := flatten(f.base)
	f.base = baseClone
	f.flushIntoBase(baseClone)
	return baseClone.Commit(ctx, sm)
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
		for ref := range x.rewrite {
			out.MarkForRewrite(ref.cand, ref.voter)
		}
		return out
	case *voterWeightFork:
		return flatten(x.voterWeightWrap)
	default:
		// There is no safe fallback here. Returning an empty view would wipe
		// every voter weight and then commit that emptiness as the new state.
		// The set of implementations is closed within this package, so reaching
		// this branch means a new one was added without updating flatten — a
		// build-time mistake that any fork/commit test will catch immediately.
		panic(fmt.Sprintf("flatten: unhandled VoterWeightView implementation %T", v))
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

// -------- initial population --------

// buildVoterWeightView constructs a fresh VoterWeightView from a snapshot of
// active native + contract-staking buckets.
//
// With the weights held as committed state this is no longer on the consensus
// path at every restart: it seeds the view before IIP-59 activates, it is the
// source the activation-time seeding draws from, and it is what a non-consensus
// audit re-derives the weights from to check the incremental path. It is not
// used to second-guess the loaded state.
//
// candidateForBucket translates a bucket's candidate identifier to the
// candidate object; nil means "candidate not found, skip the bucket".
// Self-stake bonus is gated on b.ContractAddress == "" so contract buckets
// (which always have Index = 0) don't accidentally claim the bonus.
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
	v.touched = nil
	return v
}
