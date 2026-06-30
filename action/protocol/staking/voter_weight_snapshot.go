// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/state"
)

// voterWeightSnapKey returns the state-trie key for a candidate's frozen
// voter weight blob: _voterWeightSnap (1 byte) || candID (20 bytes).
func voterWeightSnapKey(candID address.Address) []byte {
	out := make([]byte, 1+len(candID.Bytes()))
	out[0] = _voterWeightSnap
	copy(out[1:], candID.Bytes())
	return out
}

// voterWeightSnapshotBlob is the marshallable wrapper around
// stakingpb.VoterWeightSnapshot that satisfies the state.Serializer /
// state.Deserializer interface the iotex state manager expects.
type voterWeightSnapshotBlob struct {
	pb *stakingpb.VoterWeightSnapshot
}

func (b *voterWeightSnapshotBlob) Serialize() ([]byte, error) {
	if b.pb == nil {
		return proto.Marshal(&stakingpb.VoterWeightSnapshot{})
	}
	return proto.Marshal(b.pb)
}

func (b *voterWeightSnapshotBlob) Deserialize(buf []byte) error {
	pb := &stakingpb.VoterWeightSnapshot{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal voter weight snapshot")
	}
	b.pb = pb
	return nil
}

// encodeVoterWeightSnapshot serializes a sorted []voterWeight into the
// per-candidate blob format, returning both the proto object (for
// re-use by PutState's Serializer wrapper) and its marshalled bytes
// (for the unchanged-blob comparison). Sortedness is invariant from
// VoterWeightView, so the resulting blob is byte-deterministic given
// the same logical state — that's what makes the "raw-bytes equality
// skip" in SnapshotVoterWeights correct.
//
// Returns (nil, nil, nil) for empty input — the writer uses that to
// know it should delete the blob (not write an empty one).
func encodeVoterWeightSnapshot(sorted []voterWeight) (*stakingpb.VoterWeightSnapshot, []byte, error) {
	if len(sorted) == 0 {
		return nil, nil, nil
	}
	pb := &stakingpb.VoterWeightSnapshot{
		Entries: make([]*stakingpb.VoterWeightEntry, len(sorted)),
	}
	for i, vw := range sorted {
		pb.Entries[i] = &stakingpb.VoterWeightEntry{
			Voter:  vw.voter.Bytes(),
			Weight: vw.weight.Bytes(),
		}
	}
	blob, err := proto.Marshal(pb)
	if err != nil {
		return nil, nil, err
	}
	return pb, blob, nil
}

// decodeVoterWeightSnapshot reconstructs the []VoterWeight slice from the
// per-candidate blob.
func decodeVoterWeightSnapshot(buf []byte) ([]VoterWeight, error) {
	if len(buf) == 0 {
		return nil, nil
	}
	pb := &stakingpb.VoterWeightSnapshot{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal voter weight snapshot")
	}
	out := make([]VoterWeight, len(pb.Entries))
	for i, e := range pb.Entries {
		addr, err := address.FromBytes(e.Voter)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid voter address in snapshot entry %d", i)
		}
		out[i] = VoterWeight{
			Voter:  addr,
			Weight: new(big.Int).SetBytes(e.Weight),
		}
	}
	return out, nil
}

// SnapshotVoterWeights is the IIP-59 entry point called by
// poll.setCandidates at PutPollResult time. It freezes the live
// VoterWeightView's per-candidate voter contributions into a per-candidate
// blob in state, so GrantEpochReward later in the epoch reads from this
// frozen snapshot (not from the live view that may have drifted between
// PutPollResult and the epoch's last block).
//
// Incremental writes: walks every candidate the CandidateCenter knows
// about; for each, computes the new blob from the live view and compares
// to the persisted blob via raw bytes equality. If unchanged, skips the
// write. Mainnet typically sees stake activity on a small fraction of
// candidates per epoch, so the per-epoch write count is bounded by churn
// rather than total candidate count.
//
// Pre-flag callers (no IIP-59 activation) get a silent no-op via the
// voterWeights == nil check.
func (p *Protocol) SnapshotVoterWeights(sm protocol.StateManager) error {
	csr, err := ConstructBaseView(sm)
	if err != nil {
		return errors.Wrap(err, "failed to construct base view for voter weight snapshot")
	}
	vd := csr.BaseView()
	if vd.voterWeights == nil {
		return nil
	}

	// Iterate the candidate center so we cover both "candidate has voters"
	// (write/update blob) and "candidate's last voter left" (delete blob)
	// cases — the live view's map only has entries for the former.
	for _, cand := range vd.candCenter.All() {
		candID := cand.GetIdentifier()
		candIDHash := hash.BytesToHash160(candID.Bytes())

		liveVoters := readSortedLiveVoters(vd.voterWeights, candIDHash)
		newPb, newBlob, err := encodeVoterWeightSnapshot(liveVoters)
		if err != nil {
			return errors.Wrapf(err, "failed to encode voter weight snapshot for %s", candID.String())
		}

		key := voterWeightSnapKey(candID)
		oldBlob, err := readSnapshotBlob(sm, key)
		if err != nil {
			return errors.Wrapf(err, "failed to read existing snapshot for %s", candID.String())
		}

		if newPb == nil {
			if oldBlob != nil {
				if _, err := sm.DelState(
					protocol.NamespaceOption(_stakingNameSpace),
					protocol.KeyOption(key),
				); err != nil {
					return errors.Wrapf(err, "failed to delete stale snapshot for %s", candID.String())
				}
			}
			continue
		}

		if bytes.Equal(oldBlob, newBlob) {
			continue // blob unchanged — skip write
		}

		if _, err := sm.PutState(
			&voterWeightSnapshotBlob{pb: newPb},
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(key),
		); err != nil {
			return errors.Wrapf(err, "failed to write snapshot for %s", candID.String())
		}
	}
	return nil
}

// SnapshotVoterWeightsByCandidate reads the frozen per-voter contributions
// for one candidate. Used by the rewarding protocol at distribution time
// — single state lookup, no iteration. Returns nil if the candidate has
// no snapshot entry (no voters and / or feature not yet activated for
// this candidate).
func (p *Protocol) SnapshotVoterWeightsByCandidate(sr protocol.StateReader, candID address.Address) ([]VoterWeight, error) {
	key := voterWeightSnapKey(candID)
	blob := &voterWeightSnapshotBlob{}
	_, err := sr.State(blob,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(key),
	)
	switch errors.Cause(err) {
	case nil:
		return decodePbToVoterWeights(blob.pb)
	case state.ErrStateNotExist:
		return nil, nil
	default:
		return nil, errors.Wrapf(err, "failed to read voter weight snapshot for %s", candID.String())
	}
}

// readSnapshotBlob fetches the raw serialized blob bytes for unchanged-skip
// comparison. Returns (nil, nil) when there's no stored entry — the caller
// distinguishes that from a real error.
func readSnapshotBlob(sm protocol.StateReader, key []byte) ([]byte, error) {
	blob := &voterWeightSnapshotBlob{}
	_, err := sm.State(blob,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(key),
	)
	switch errors.Cause(err) {
	case nil:
		return blob.Serialize()
	case state.ErrStateNotExist:
		return nil, nil
	default:
		return nil, err
	}
}

// readSortedLiveVoters pulls the per-(cand, voter) entries for a single
// candidate from the live VoterWeightView, in sorted-by-voter order. The
// internal voterWeight slice the view returns is already package-private
// and sorted; this helper just re-projects without copying weights.
func readSortedLiveVoters(view VoterWeightView, candID hash.Hash160) []voterWeight {
	// VoterWeightsByCandidate already gives us a sorted copy.
	return view.VoterWeightsByCandidate(candID)
}

// decodePbToVoterWeights converts a stakingpb.VoterWeightSnapshot to the
// public VoterWeight slice the rewarding package consumes.
func decodePbToVoterWeights(pb *stakingpb.VoterWeightSnapshot) ([]VoterWeight, error) {
	if pb == nil || len(pb.Entries) == 0 {
		return nil, nil
	}
	out := make([]VoterWeight, len(pb.Entries))
	for i, e := range pb.Entries {
		addr, err := address.FromBytes(e.Voter)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid voter address in snapshot entry %d", i)
		}
		out[i] = VoterWeight{
			Voter:  addr,
			Weight: new(big.Int).SetBytes(e.Weight),
		}
	}
	return out, nil
}
