// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/delegateprofile"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/state"
)

// CandidatePollSnapshot is the frozen per-candidate view that IIP-59's
// rewarding path consumes at each epoch close. It is written once per epoch
// by FreezePollSnapshot (called from the poll layer's PutPollResult) and
// never mutated during the epoch — the epoch-boundary freeze is the whole
// point, so any mid-epoch change to the DelegateProfile contract or to the
// candidate's opt-in flag does not retroactively re-split rewards that
// have already begun accruing.
type CandidatePollSnapshot struct {
	// BlockCommissionBasisPoints is the delegate's take of block rewards, in
	// basis points [0, 10000]. Zero when Registered is false.
	BlockCommissionBasisPoints uint64
	// EpochCommissionBasisPoints is the delegate's take of epoch rewards, in
	// basis points [0, 10000]. Zero when Registered is false.
	EpochCommissionBasisPoints uint64
	// Registered is true when the DelegateProfile contract returned both
	// portion fields as non-empty bytes at snapshot time. False ⇒ the two
	// commission rates above are meaningless and the caller MUST fall back
	// to the legacy Hermes distribution path.
	Registered bool
	// VoterRewardOnchainOptIn is the value of staking.Candidate's opt-in
	// flag at snapshot time. Rewarding reads THIS, not the live Candidate,
	// which is what makes the opt-in transition delayed one epoch.
	VoterRewardOnchainOptIn bool
	// Entries is the per-voter aggregated weight list, sorted ascending by
	// Voter bytes. Empty in the initial IIP-59 skeleton PR; a follow-up
	// fills in the actual voter-weight source. Downstream rewarding treats
	// an empty list as "no voters known — pay full amount to the delegate
	// as commission" via its existing degenerate branch.
	Entries []VoterWeight
}

// VoterWeight is one entry in CandidatePollSnapshot.Entries.
type VoterWeight struct {
	Voter  address.Address
	Weight *big.Int
}

// candidatePollSnapshotBlob is the state.Serializer / state.Deserializer
// wrapper around stakingpb.CandidatePollSnapshot. Kept package-private —
// callers work with CandidatePollSnapshot.
type candidatePollSnapshotBlob struct {
	pb *stakingpb.CandidatePollSnapshot
}

// Serialize implements state.Serializer.
func (b *candidatePollSnapshotBlob) Serialize() ([]byte, error) {
	if b.pb == nil {
		return proto.Marshal(&stakingpb.CandidatePollSnapshot{})
	}
	return proto.Marshal(b.pb)
}

// Deserialize implements state.Deserializer.
func (b *candidatePollSnapshotBlob) Deserialize(buf []byte) error {
	pb := &stakingpb.CandidatePollSnapshot{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate poll snapshot")
	}
	b.pb = pb
	return nil
}

// candidatePollSnapshotKey returns the state-trie key for a candidate's
// frozen poll snapshot: {_candidatePollSnapshot} || candID.Bytes(). Namespace
// is _stakingNameSpace (see protocol.go). Mirrors the layout of the other
// candidate-scoped keys in this package.
func candidatePollSnapshotKey(candID address.Address) []byte {
	out := make([]byte, 1, 1+len(candID.Bytes()))
	out[0] = _candidatePollSnapshot
	return append(out, candID.Bytes()...)
}

// toBlob converts a CandidatePollSnapshot into the wire form for PutState.
func (s *CandidatePollSnapshot) toBlob() *candidatePollSnapshotBlob {
	pb := &stakingpb.CandidatePollSnapshot{
		BlockCommissionBasisPoints: s.BlockCommissionBasisPoints,
		EpochCommissionBasisPoints: s.EpochCommissionBasisPoints,
		Registered:                 s.Registered,
		VoterRewardOnchainOptIn:    s.VoterRewardOnchainOptIn,
	}
	if len(s.Entries) > 0 {
		pb.Entries = make([]*stakingpb.VoterWeightEntry, 0, len(s.Entries))
		for _, e := range s.Entries {
			var weight []byte
			if e.Weight != nil {
				weight = e.Weight.Bytes()
			}
			pb.Entries = append(pb.Entries, &stakingpb.VoterWeightEntry{
				Voter:  e.Voter.Bytes(),
				Weight: weight,
			})
		}
	}
	return &candidatePollSnapshotBlob{pb: pb}
}

// fromBlob converts a decoded blob back into a CandidatePollSnapshot.
func fromBlob(b *candidatePollSnapshotBlob) (*CandidatePollSnapshot, error) {
	if b == nil || b.pb == nil {
		return &CandidatePollSnapshot{}, nil
	}
	out := &CandidatePollSnapshot{
		BlockCommissionBasisPoints: b.pb.GetBlockCommissionBasisPoints(),
		EpochCommissionBasisPoints: b.pb.GetEpochCommissionBasisPoints(),
		Registered:                 b.pb.GetRegistered(),
		VoterRewardOnchainOptIn:    b.pb.GetVoterRewardOnchainOptIn(),
	}
	if entries := b.pb.GetEntries(); len(entries) > 0 {
		out.Entries = make([]VoterWeight, 0, len(entries))
		for _, e := range entries {
			addr, err := address.FromBytes(e.GetVoter())
			if err != nil {
				return nil, errors.Wrap(err, "invalid voter address bytes in poll snapshot")
			}
			out.Entries = append(out.Entries, VoterWeight{
				Voter:  addr,
				Weight: new(big.Int).SetBytes(e.GetWeight()),
			})
		}
	}
	return out, nil
}

// FreezePollSnapshot writes a CandidatePollSnapshot for each candidate at
// PutPollResult. This is the *only* writer of the snapshot; rewarding is a
// pure reader via PollSnapshotFor.
//
// When bridge is nil (DelegateProfile contract not configured for this
// network), the commission-rate freeze is skipped: every snapshot carries
// Registered=false and both rate fields zero, but the opt-in flag is still
// captured from the live staking.Candidate. Downstream rewarding will then
// take the legacy Hermes path because Registered=false.
//
// Any per-delegate error (bridge failure, invalid identity) aborts the whole
// snapshot write. A partial map here would leak delegates onto the wrong
// reward path for an entire epoch, so we prefer failing the block.
func FreezePollSnapshot(
	ctx context.Context,
	sm protocol.StateManager,
	candidates state.CandidateList,
	bridge *delegateprofile.Bridge,
	reader delegateprofile.ContractReader,
) error {
	// Parse candidate addresses once; both the bridge call and the snapshot
	// write use them.
	ids := make([]address.Address, 0, len(candidates))
	for _, c := range candidates {
		if c == nil {
			return errors.New("staking: nil candidate in poll list")
		}
		id, err := address.FromString(c.Address)
		if err != nil {
			return errors.Wrapf(err, "staking: invalid candidate address %q", c.Address)
		}
		ids = append(ids, id)
	}

	var rates map[string]*delegateprofile.CommissionRates
	if bridge != nil {
		if reader == nil {
			return errors.New("staking: nil ContractReader with non-nil DelegateProfile bridge")
		}
		var err error
		rates, err = bridge.Snapshot(ctx, reader, ids)
		if err != nil {
			return errors.Wrap(err, "staking: DelegateProfile snapshot failed")
		}
	}

	for _, id := range ids {
		optIn, err := readLiveOptIn(sm, id)
		if err != nil {
			return errors.Wrapf(err, "staking: read opt-in for candidate %s", id.String())
		}
		snap := &CandidatePollSnapshot{
			VoterRewardOnchainOptIn: optIn,
		}
		if r, ok := rates[id.String()]; ok && r != nil && r.Registered {
			snap.BlockCommissionBasisPoints = r.BlockCommissionBasisPoints
			snap.EpochCommissionBasisPoints = r.EpochCommissionBasisPoints
			snap.Registered = true
		}
		if _, err := sm.PutState(
			snap.toBlob(),
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(candidatePollSnapshotKey(id)),
		); err != nil {
			return errors.Wrapf(err, "staking: write poll snapshot for candidate %s", id.String())
		}
	}
	return nil
}

// PollSnapshotFor returns the frozen snapshot written at the most recent
// PutPollResult for the given candidate identity. Returns
// (nil, state.ErrStateNotExist) when no snapshot has been written (pre-fork
// / config-off epochs), in which case the caller MUST fall back to the
// legacy path.
func PollSnapshotFor(sr protocol.StateReader, candID address.Address) (*CandidatePollSnapshot, error) {
	if candID == nil {
		return nil, errors.New("staking: nil candidate identity")
	}
	blob := &candidatePollSnapshotBlob{}
	if _, err := sr.State(
		blob,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(candidatePollSnapshotKey(candID)),
	); err != nil {
		return nil, err
	}
	return fromBlob(blob)
}

// readLiveOptIn reads the persistent staking.Candidate at candID and returns
// its VoterRewardOnchainOptIn flag. Returns (false, nil) if the Candidate is
// absent — that would indicate an upstream data mismatch (poll list names a
// candidate that has no staking record), but we degrade to opt-out rather
// than fail the block so the whole chain doesn't wedge on one bad entry.
func readLiveOptIn(sr protocol.StateReader, candID address.Address) (bool, error) {
	var c Candidate
	if _, err := sr.State(
		&c,
		protocol.NamespaceOption(_candidateNameSpace),
		protocol.KeyOption(candID.Bytes()),
	); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return false, nil
		}
		return false, err
	}
	return c.VoterRewardOnchainOptIn, nil
}
