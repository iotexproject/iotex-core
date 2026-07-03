// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/state"
)

// VoterWeight is a public projection of a single per-candidate snapshot entry:
// one voter's aggregated vote weight across all buckets (native + contract)
// tied to that candidate, frozen at PutPollResult time. Rewarding consumes
// this at GrantEpochReward to split the epoch reward between voters.
type VoterWeight struct {
	Voter  address.Address
	Weight *big.Int
}

// snapshotEntry mirrors VoterWeight but stays package-private so we can
// enforce sorting and encoding invariants inside this file.
type snapshotEntry struct {
	voter  address.Address
	weight *big.Int
}

// voterWeightSnapKey returns the state-trie key for a candidate's frozen
// voter weight blob: _voterWeightSnap (1 byte) || candID.Bytes() (20 bytes).
// The 21-byte layout mirrors the other candidate-scoped keys in this
// package (see _voterIndex / _candIndex prefixes in protocol.go).
func voterWeightSnapKey(candID address.Address) []byte {
	out := make([]byte, 1, 1+len(candID.Bytes()))
	out[0] = _voterWeightSnap
	return append(out, candID.Bytes()...)
}

// voterWeightSnapshotBlob wraps stakingpb.VoterWeightSnapshot so it satisfies
// the state.Serializer / state.Deserializer contract the state manager
// expects on PutState / State roundtrips.
type voterWeightSnapshotBlob struct {
	pb *stakingpb.VoterWeightSnapshot
}

// Serialize implements state.Serializer.
func (b *voterWeightSnapshotBlob) Serialize() ([]byte, error) {
	if b.pb == nil {
		return proto.Marshal(&stakingpb.VoterWeightSnapshot{})
	}
	return proto.Marshal(b.pb)
}

// Deserialize implements state.Deserializer.
func (b *voterWeightSnapshotBlob) Deserialize(buf []byte) error {
	pb := &stakingpb.VoterWeightSnapshot{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal voter weight snapshot")
	}
	b.pb = pb
	return nil
}

// encodeVoterWeightSnapshot serializes a []snapshotEntry into the per-candidate
// blob format, returning both the pb (for PutState wrapper) and its marshalled
// bytes (for the "unchanged blob → skip write" comparison in the writer).
//
// The caller is responsible for passing entries pre-sorted by voter address
// (big-endian bytes). That ordering is the load-bearing invariant behind
// byte-equal-skip: identical logical state MUST produce identical marshalled
// bytes.
//
// Returns (nil, nil, nil) when the candidate has no voter entries — writers
// interpret this as "delete the blob," not "write empty."
func encodeVoterWeightSnapshot(sorted []snapshotEntry) (*stakingpb.VoterWeightSnapshot, []byte, error) {
	if len(sorted) == 0 {
		return nil, nil, nil
	}
	pb := &stakingpb.VoterWeightSnapshot{
		Entries: make([]*stakingpb.VoterWeightEntry, len(sorted)),
	}
	for i, e := range sorted {
		pb.Entries[i] = &stakingpb.VoterWeightEntry{
			Voter:  e.voter.Bytes(),
			Weight: e.weight.Bytes(),
		}
	}
	blob, err := proto.Marshal(pb)
	if err != nil {
		return nil, nil, err
	}
	return pb, blob, nil
}

// decodeVoterWeightSnapshot reconstructs the public []VoterWeight slice
// from a persisted per-candidate blob.
func decodeVoterWeightSnapshot(pb *stakingpb.VoterWeightSnapshot) ([]VoterWeight, error) {
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

// VoterWeightsFromSnapshot reads the frozen per-voter weights for a single
// candidate. Rewarding uses this at GrantEpochReward; each call is a single
// state.State lookup on _voterWeightSnap namespace. Returns (nil, nil) when
// the candidate has no snapshot entry — either not yet activated, or all
// voters left. Errors other than state.ErrStateNotExist propagate.
func VoterWeightsFromSnapshot(sr protocol.StateReader, candID address.Address) ([]VoterWeight, error) {
	blob := &voterWeightSnapshotBlob{}
	_, err := sr.State(blob,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(voterWeightSnapKey(candID)),
	)
	switch errors.Cause(err) {
	case nil:
		return decodeVoterWeightSnapshot(blob.pb)
	case state.ErrStateNotExist:
		return nil, nil
	default:
		return nil, errors.Wrapf(err, "failed to read voter weight snapshot for %s", candID.String())
	}
}

// readSnapshotBlobBytes returns the raw marshalled bytes of an existing
// per-candidate blob, for the byte-equality comparison in the writer.
// (nil, nil) means "no blob stored" — distinct from an actual empty blob,
// which the encoder does not produce.
func readSnapshotBlobBytes(sm protocol.StateReader, key []byte) ([]byte, error) {
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

// SnapshotForEpochReward is the IIP-59 entry point invoked by
// poll.setCandidates at PutPollResult time. For each candidate in the
// next-epoch active delegate list, it:
//
//  1. copies the delegate's latest CommissionRate from the live
//     staking.Candidate into the caller's state.Candidate — the caller
//     then persists this via sm.PutState(&candidates, ...NxtCandidateKey...),
//     freezing the rate for the next epoch.
//  2. reads the per-voter weights out of the live viewData.voterWeights
//     view (kept incrementally consistent by native handlers and the
//     contract-staking-event sink) and writes an incremental per-candidate
//     blob to the _voterWeightSnap namespace.
//
// The view is the single source of truth here: we never scan buckets and
// never read the contract-staking indexer db from this path, so this call
// stays state.db-native.
//
// Only candidates in the passed-in list get voter-weight snapshots — the
// non-top-N delegates cannot earn reward this epoch anyway. Blobs for
// delegates that dropped out of the top-N are not deleted here (they
// simply become stale and are ignored by GrantEpochReward, which only
// looks up delegates it actually pays).
//
// This is a no-op when the caller's featureCtx has voter reward
// distribution disabled (pre-fork). Safe to call unconditionally from the
// hot path; the guard belongs here to keep poll's call site clean.
func (p *Protocol) SnapshotForEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
	cands state.CandidateList,
) error {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.NoVoterRewardDistribution {
		return nil
	}
	if len(cands) == 0 {
		return nil
	}

	csm, err := NewCandidateStateManager(sm)
	if err != nil {
		return errors.Wrap(err, "failed to construct candidate state manager for voter reward snapshot")
	}

	view, err := sm.ReadView(_protocolID)
	if err != nil {
		return errors.Wrap(err, "failed to read staking view for voter reward snapshot")
	}
	vd, ok := view.(*viewData)
	if !ok || vd.voterWeights == nil {
		return errors.New("voter weight view is not initialized")
	}

	for _, c := range cands {
		if c == nil || c.Identity == "" {
			continue
		}
		candID, err := address.FromString(c.Identity)
		if err != nil {
			return errors.Wrapf(err, "invalid candidate identity %s", c.Identity)
		}
		stakingCand := csm.GetByIdentifier(candID)
		if stakingCand == nil {
			// Candidate exists in poll's list but not in staking center — either
			// removed since the last epoch, or a legacy-only entry. Skip; no
			// commissionRate to freeze and no view entry to read.
			continue
		}

		// (1) freeze commissionRate into state.Candidate; poll will PutState.
		c.CommissionRate = stakingCand.CommissionRate

		// (2) read weights straight out of the live view. view.Weights returns
		// entries pre-sorted by voter address, matching the encoder's
		// determinism contract.
		weights := vd.voterWeights.Weights(candID)
		entries := make([]snapshotEntry, 0, len(weights))
		for _, w := range weights {
			if w.Weight == nil || w.Weight.Sign() == 0 {
				continue
			}
			entries = append(entries, snapshotEntry{voter: w.Voter, weight: new(big.Int).Set(w.Weight)})
		}
		if err := writeVoterWeightSnapshot(sm, candID, entries); err != nil {
			return errors.Wrapf(err, "failed to write voter weight snapshot for %s", candID.String())
		}
	}
	return nil
}

// writeVoterWeightSnapshot performs the incremental write for one candidate:
// encode the entries, compare to the existing blob via raw bytes, and either
// skip (byte-equal), delete (empty), or overwrite.
func writeVoterWeightSnapshot(sm protocol.StateManager, candID address.Address, entries []snapshotEntry) error {
	key := voterWeightSnapKey(candID)
	newPb, newBlob, err := encodeVoterWeightSnapshot(entries)
	if err != nil {
		return err
	}
	oldBlob, err := readSnapshotBlobBytes(sm, key)
	if err != nil {
		return err
	}
	if newPb == nil {
		if oldBlob != nil {
			if _, err := sm.DelState(
				protocol.NamespaceOption(_stakingNameSpace),
				protocol.KeyOption(key),
			); err != nil {
				return errors.Wrap(err, "failed to delete stale voter weight snapshot")
			}
		}
		return nil
	}
	if bytes.Equal(oldBlob, newBlob) {
		return nil
	}
	if _, err := sm.PutState(
		&voterWeightSnapshotBlob{pb: newPb},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(key),
	); err != nil {
		return errors.Wrap(err, "failed to write voter weight snapshot")
	}
	return nil
}
