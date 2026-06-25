// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
)

// _voterWeightsKey is the single state-trie key under StakingNamespace where
// IIP-59's view digest lives. The 1-byte namespace tag _voterWeights = 5 is
// reserved in protocol.go.
var _voterWeightsKey = []byte{_voterWeights}

// voterWeightDigest is the serialized form of VoterWeightView.Hash() —
// one 32-byte value per chain, rewritten only when the view is dirty at
// block commit time. Deserialize requires the buffer to be exactly 32
// bytes so a corrupted record fails loudly rather than silently
// producing a zero hash.
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

// ensureVoterWeightView populates viewData.voterWeights when it has not yet
// been built (first block after the feature flag activates, or restart
// before the incremental hooks have run). On the first non-empty build it
// verifies the rebuilt hash against the digest persisted under
// _voterWeightsKey; a mismatch surfaces as a loud error so a desync
// between bucket state and view state cannot silently pass into block
// production.
//
// Subsequent calls within the same flag-on chain are no-ops: voterWeights
// is non-nil and gets maintained incrementally by the handler hooks.
func (p *Protocol) ensureVoterWeightView(ctx context.Context, sm protocol.StateManager) error {
	csr, err := ConstructBaseView(sm)
	if err != nil {
		return err
	}
	vd := csr.BaseView()
	if vd.voterWeights != nil {
		return nil
	}

	allBuckets, _, err := newCandidateStateReader(sm).NativeBuckets()
	if err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return errors.Wrap(err, "failed to read native buckets")
	}
	candCenter := vd.candCenter
	view := buildVoterWeightView(allBuckets, func(b *VoteBucket) *Candidate {
		return candCenter.GetByIdentifier(b.Candidate)
	}, p.config.VoteWeightCalConsts)

	// Verify against the persisted digest, if any. If no digest record
	// exists yet (first-ever activation), the rebuilt view is the
	// authoritative starting point and we write its hash on the next
	// dirty commit.
	persisted, err := readVoterWeightDigest(sm)
	switch errors.Cause(err) {
	case nil:
		if persisted != view.Hash() {
			return errors.Errorf(
				"voter weight view digest mismatch: rebuilt %x vs persisted %x — staking view is divergent",
				view.Hash(), persisted,
			)
		}
	case state.ErrStateNotExist:
		// First activation: mark the view dirty so the next Commit
		// persists the initial digest.
		view.dirty = true
	default:
		return errors.Wrap(err, "failed to read persisted voter weight digest")
	}

	vd.voterWeights = view
	return nil
}

// applyVoterWeightDelta is the single entry point every staking handler
// uses to keep the IIP-59 VoterWeightView in sync with on-chain bucket
// changes. It is a no-op when the feature flag has not activated yet (view
// remains nil) and when delta is zero, so callers can wire it next to any
// existing candidate.AddVote / candidate.SubVote site without first
// checking the flag.
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
// Maintenance model: incrementally updated by every staking handler that
// changes a bucket's contribution to a candidate (CreateStake, Unstake,
// Restake, ChangeCandidate, TransferStake, DepositToStake, contract-staking
// events, etc.). The view sits inside viewData and follows the same
// Fork/Snapshot/Revert/Commit lifecycle as bucketPool.
//
// Determinism: per-candidate voter slices are kept sorted by voter address so
// callers can iterate them in a stable order — distributeVoterReward must
// never iterate a Go map to compute receipt-log order or state writes
// (see #4811 review #2 for the bug class).
type VoterWeightView struct {
	byCandidate map[hash.Hash160]*candidateVoterEntry
	dirty       bool
}

// candidateVoterEntry holds the per-(candidate, voter) weighted votes for
// one candidate. The slice is sorted by voter address (lexicographic on
// 20-byte hash160); the map is a fast lookup into the slice.
type candidateVoterEntry struct {
	sorted []voterWeight
	index  map[hash.Hash160]int
}

// voterWeight is a single (voter, weighted-votes) pair belonging to some
// candidate. Weight is the value returned by CalculateVoteWeight for the
// bucket that gave rise to this contribution. Multiple buckets from the
// same voter to the same candidate are aggregated into a single entry by
// VoterWeightView.Apply, so the protocol distributes per voter, not per
// bucket — this avoids per-bucket rounding loss.
type voterWeight struct {
	voter  address.Address
	weight *big.Int
}

// NewVoterWeightView returns an empty view.
func NewVoterWeightView() *VoterWeightView {
	return &VoterWeightView{
		byCandidate: make(map[hash.Hash160]*candidateVoterEntry),
	}
}

// Apply adjusts the weight that voter contributes to candidate by delta.
// A positive delta increases the voter's weight (e.g. CreateStake adds a
// new bucket's vote weight); a negative delta decreases it (Unstake,
// ChangeCandidate-from). When the aggregated weight reaches zero the voter
// entry is removed; when the candidate's entry becomes empty it is removed
// from the view too.
//
// Apply is the single mutation entry point for the view — all handler
// hooks must funnel through here so that updates remain consistent with
// the snapshot/revert machinery.
func (v *VoterWeightView) Apply(candID hash.Hash160, voter address.Address, delta *big.Int) {
	if delta == nil || delta.Sign() == 0 {
		return
	}
	v.dirty = true
	entry, ok := v.byCandidate[candID]
	if !ok {
		// Negative delta against an empty candidate is a programming error
		// upstream — the handler computed a withdrawal but the view does
		// not know about the voter. We silently treat as no-op rather than
		// crash; the view-hash check at restart will catch any divergence
		// (see Hash()) and surface it loudly there.
		if delta.Sign() < 0 {
			return
		}
		entry = &candidateVoterEntry{
			sorted: nil,
			index:  make(map[hash.Hash160]int),
		}
		v.byCandidate[candID] = entry
	}

	voterID := hash.BytesToHash160(voter.Bytes())
	if slot, ok := entry.index[voterID]; ok {
		// Existing voter — adjust weight in place.
		newWeight := new(big.Int).Add(entry.sorted[slot].weight, delta)
		if newWeight.Sign() <= 0 {
			// Voter has no more weight on this candidate — remove the entry.
			entry.removeAt(slot)
			if len(entry.sorted) == 0 {
				delete(v.byCandidate, candID)
			}
			return
		}
		entry.sorted[slot].weight = newWeight
		return
	}

	if delta.Sign() < 0 {
		// Same rationale as the missing-candidate branch above.
		return
	}
	entry.insertSorted(voter, voterID, new(big.Int).Set(delta))
}

// VoterWeightsByCandidate returns the per-voter weight contributions for
// the given candidate, sorted by voter address. The returned slice is a
// shallow copy: callers may iterate freely without affecting view state,
// but must not mutate the *big.Int weights in place (treat as read-only).
// Returns nil if the candidate has no active voters.
func (v *VoterWeightView) VoterWeightsByCandidate(candID hash.Hash160) []voterWeight {
	entry, ok := v.byCandidate[candID]
	if !ok || len(entry.sorted) == 0 {
		return nil
	}
	out := make([]voterWeight, len(entry.sorted))
	for i, vw := range entry.sorted {
		out[i] = voterWeight{voter: vw.voter, weight: new(big.Int).Set(vw.weight)}
	}
	return out
}

// Hash returns a deterministic 32-byte digest of the entire view. It is
// constructed by iterating candidates in sorted hash160 order and, within
// each candidate, walking the already-sorted voter slice — so two nodes
// that observed the same sequence of Apply calls (in any interleaving
// permissible by the staking handlers, which are themselves deterministic)
// produce identical hashes.
//
// The hash is persisted at the _voterWeights namespace tag on every block
// commit and re-checked at restart against a rebuilt-from-buckets view.
func (v *VoterWeightView) Hash() hash.Hash256 {
	if len(v.byCandidate) == 0 {
		return hash.ZeroHash256
	}
	candIDs := make([]hash.Hash160, 0, len(v.byCandidate))
	for id := range v.byCandidate {
		candIDs = append(candIDs, id)
	}
	sort.Slice(candIDs, func(i, j int) bool {
		return bytes.Compare(candIDs[i][:], candIDs[j][:]) < 0
	})

	var buf bytes.Buffer
	scratch := make([]byte, 8)
	for _, candID := range candIDs {
		buf.Write(candID[:])
		entry := v.byCandidate[candID]
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

// Clone returns a deep copy of the view, suitable for Fork().
func (v *VoterWeightView) Clone() *VoterWeightView {
	if v == nil {
		return nil
	}
	out := NewVoterWeightView()
	out.dirty = v.dirty
	for candID, entry := range v.byCandidate {
		clone := &candidateVoterEntry{
			sorted: make([]voterWeight, len(entry.sorted)),
			index:  make(map[hash.Hash160]int, len(entry.index)),
		}
		for i, vw := range entry.sorted {
			clone.sorted[i] = voterWeight{
				voter:  vw.voter,
				weight: new(big.Int).Set(vw.weight),
			}
		}
		for k, slot := range entry.index {
			clone.index[k] = slot
		}
		out.byCandidate[candID] = clone
	}
	return out
}

// IsEmpty reports whether the view has no candidates. Useful for restart
// hash checks to distinguish "view was never built" from "view is zero hash".
func (v *VoterWeightView) IsEmpty() bool {
	return v == nil || len(v.byCandidate) == 0
}

// buildVoterWeightView constructs a fresh VoterWeightView from a snapshot of
// active native + contract-staking buckets at the given height. Used by
// staking.Protocol.CreatePreStates on the block that first activates the
// IIP-59 feature flag (one-shot full scan; thereafter the view is
// maintained incrementally by the handler hooks) and at restart to
// reconstruct the view and verify it against the persisted digest.
//
// candidateLookup translates a bucket's candidate operator/identifier to
// the candidate's stable identifier address used as the view's primary
// key. selfStakeBucketIdx allows the canonical IIP-59 self-stake bonus to
// apply only to native self-stake buckets (the PoC #4811 review #5 bug
// was to apply it to any bucket with the same index, including contract
// buckets which always have Index = 0). The caller is responsible for
// passing nil contractIndexers when none are configured.
func buildVoterWeightView(
	allBuckets []*VoteBucket,
	candidateForBucket func(*VoteBucket) *Candidate,
	consts genesis.VoteWeightCalConsts,
) *VoterWeightView {
	v := NewVoterWeightView()
	for _, b := range allBuckets {
		if b == nil || b.isUnstaked() {
			continue
		}
		cand := candidateForBucket(b)
		if cand == nil {
			// Bucket points to a candidate we don't know about — skip.
			// Apply has the same defensive behavior, so this is consistent.
			continue
		}
		// Self-stake bonus applies only to native self-stake buckets — a
		// contract bucket with the same Index (commonly 0) must not get
		// the bonus. PoC #4811 review finding #5 was exactly this bug.
		isSelfStake := b.ContractAddress == "" && b.Index == cand.SelfStakeBucketIdx
		w := CalculateVoteWeight(consts, b, isSelfStake)
		if w.Sign() == 0 {
			continue
		}
		v.Apply(hash.BytesToHash160(cand.GetIdentifier().Bytes()), b.Owner, w)
	}
	// Initial build is by definition consistent with the on-disk state at
	// this height, so commit can be a no-op until the next mutation.
	v.MarkClean()
	return v
}

// IsDirty reports whether the view has been mutated since the last
// MarkClean. viewData.IsDirty consults this so that block commits know
// whether the persisted hash needs to be rewritten.
func (v *VoterWeightView) IsDirty() bool {
	return v != nil && v.dirty
}

// MarkClean clears the dirty flag. Called from viewData.Commit after the
// new hash has been persisted.
func (v *VoterWeightView) MarkClean() {
	if v != nil {
		v.dirty = false
	}
}

// insertSorted inserts (voter, weight) into the entry at the position
// that keeps entry.sorted sorted by voter address. The entry's index map is
// rebuilt for affected slots (everything from insertion point onwards).
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

// removeAt removes the entry at the given slot, compacts the slice, and
// rebuilds the index map for everything from slot onwards.
func (e *candidateVoterEntry) removeAt(slot int) {
	id := hash.BytesToHash160(e.sorted[slot].voter.Bytes())
	delete(e.index, id)
	e.sorted = append(e.sorted[:slot], e.sorted[slot+1:]...)
	for i := slot; i < len(e.sorted); i++ {
		shiftedID := hash.BytesToHash160(e.sorted[i].voter.Bytes())
		e.index[shiftedID] = i
	}
}
