// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/delegateprofile"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/distributedlog"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// CandidatePollSnapshot is the frozen per-candidate view that IIP-59's
// rewarding path consumes at each epoch close. It is written once per epoch
// by FreezePollSnapshot (called from the poll layer's PutPollResult) and
// never mutated during the reward era. Mid-era DelegateProfile changes do not
// retroactively re-split rewards that have already begun accruing.
type CandidatePollSnapshot struct {
	// BlockCommissionBasisPoints is the delegate's take of block rewards, in
	// basis points [0, 10000]. Zero when Registered is false.
	BlockCommissionBasisPoints uint64
	// EpochCommissionBasisPoints is the delegate's take of epoch rewards, in
	// basis points [0, 10000]. Zero when Registered is false.
	EpochCommissionBasisPoints uint64
	// Registered is true when the DelegateProfile contract returned both
	// portion fields as non-empty bytes at snapshot time. False means both
	// commission rates default to zero and the full reward goes to voters.
	Registered bool
	// Entries is the per-voter aggregated weight list, sorted ascending by
	// Voter bytes (invariant maintained by VoterWeightView). Downstream
	// rewarding treats an empty list as "no voters known — pay full amount
	// to the delegate as commission" via its existing degenerate branch;
	// FreezePollSnapshot populates it from the live VoterWeightView, so
	// this only ever ends up empty when the view is nil (pre-fork setups)
	// or the candidate genuinely has zero non-self-stake buckets.
	Entries []VoterWeight
	// Cached metadata keeps each continuation block proportional to its voter
	// window. HasWeightedEntries distinguishes a valid index 0 from no weights.
	TotalWeight        *big.Int
	SnapshotHash       hash.Hash256
	LastWeightedIndex  uint32
	HasWeightedEntries bool
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

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (b *candidatePollSnapshotBlob) Encode() (systemcontracts.GenericValue, error) {
	data, err := b.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (b *candidatePollSnapshotBlob) Decode(v systemcontracts.GenericValue) error {
	return b.Deserialize(v.PrimaryData)
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
	populateSnapshotMetadata(s)
	pb := &stakingpb.CandidatePollSnapshot{
		BlockCommissionBasisPoints: s.BlockCommissionBasisPoints,
		EpochCommissionBasisPoints: s.EpochCommissionBasisPoints,
		Registered:                 s.Registered,
		TotalWeight:                s.TotalWeight.Bytes(),
		SnapshotHash:               s.SnapshotHash[:],
		LastWeightedIndex:          s.LastWeightedIndex,
		HasWeightedEntries:         s.HasWeightedEntries,
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
		TotalWeight:                new(big.Int).SetBytes(b.pb.GetTotalWeight()),
		SnapshotHash:               hash.BytesToHash256(b.pb.GetSnapshotHash()),
		LastWeightedIndex:          b.pb.GetLastWeightedIndex(),
		HasWeightedEntries:         b.pb.GetHasWeightedEntries(),
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
	if len(b.pb.GetSnapshotHash()) != len(hash.Hash256{}) {
		populateSnapshotMetadata(out)
	}
	return out, nil
}

func populateSnapshotMetadata(s *CandidatePollSnapshot) {
	if s == nil {
		return
	}
	voters := make([]address.Address, len(s.Entries))
	weights := make([]*big.Int, len(s.Entries))
	totalWeight := new(big.Int)
	s.HasWeightedEntries = false
	s.LastWeightedIndex = 0
	for i, entry := range s.Entries {
		voters[i] = entry.Voter
		weight := new(big.Int)
		if entry.Weight != nil {
			weight.Set(entry.Weight)
		}
		weights[i] = weight
		totalWeight.Add(totalWeight, weight)
		if weight.Sign() > 0 {
			s.HasWeightedEntries = true
			s.LastWeightedIndex = uint32(i)
		}
	}
	s.TotalWeight = totalWeight
	s.SnapshotHash = distributedlog.SnapshotHash(voters, weights)
}

// FreezePollSnapshot writes a CandidatePollSnapshot for each candidate at
// PutPollResult. This is the *only* writer of the snapshot; rewarding is a
// pure reader via PollSnapshotFor.
//
// When bridge is nil (DelegateProfile contract not configured for this
// network), every snapshot carries Registered=false and both rate fields zero,
// which is the post-fork default of sending the full reward to voters.
//
// A per-delegate bridge read failure (malformed profile, RPC/EVM error) is
// absorbed by the bridge itself: the affected delegate lands with
// Registered=false and rewarding uses the all-to-voters default. This
// prevents one bad on-chain profile from deterministically halting the
// chain at every epoch boundary — same state ⇒ same fallback on every
// validator ⇒ no fork.
//
// Only wiring-level errors (invalid candidate address, nil-bridge-with-nil-
// reader, PutState failure) still abort the whole write.
func FreezePollSnapshot(
	ctx context.Context,
	sm protocol.StateManager,
	candidates state.CandidateList,
	bridge *delegateprofile.Bridge,
	reader delegateprofile.ContractReader,
) error {
	// Parse candidate identities once; both the bridge call and the snapshot
	// write use them.
	ids := make([]address.Address, 0, len(candidates))
	for _, c := range candidates {
		if c == nil {
			return errors.New("staking: nil candidate in poll list")
		}
		identity := c.Identity
		if identity == "" {
			// Legacy poll lists before identity storage used Address as the
			// candidate identifier. IIP-59-era native staking lists always
			// populate Identity.
			identity = c.Address
		}
		id, err := address.FromString(identity)
		if err != nil {
			return errors.Wrapf(err, "staking: invalid candidate identity %q", identity)
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

	// The VoterWeightView is the source of truth for per-(candidate, voter)
	// aggregated weights, kept live by applyVoterWeightDelta hooks on every
	// bucket mutation. When it is missing (pre-fork setups that skip
	// Protocol.Start, or tests that write the view directly without an
	// install step) the freezer degrades gracefully: Entries is left nil on
	// every snapshot and rewarding keeps the voter pool pending until a later
	// era has an eligible snapshot. When present, we
	// copy VoterWeightsByCandidate output directly; the view already sorts
	// entries by voter bytes, so no re-sort here.
	vw := voterWeightsFromSM(sm)

	for _, id := range ids {
		snap := &CandidatePollSnapshot{}
		if r, ok := rates[id.String()]; ok && r != nil && r.Registered {
			snap.BlockCommissionBasisPoints = r.BlockCommissionBasisPoints
			snap.EpochCommissionBasisPoints = r.EpochCommissionBasisPoints
			snap.Registered = true
		}
		if vw != nil {
			weights := vw.VoterWeightsByCandidate(hash.BytesToHash160(id.Bytes()))
			if len(weights) > 0 {
				snap.Entries = make([]VoterWeight, 0, len(weights))
				for _, w := range weights {
					snap.Entries = append(snap.Entries, VoterWeight{
						Voter:  w.voter,
						Weight: new(big.Int).Set(w.weight),
					})
				}
			}
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

// voterWeightsFromSM returns the live VoterWeightView from the staking view,
// or nil when the view isn't installed. Read-only — callers must not Apply
// through it. Wiring-level failures (view type mismatch) are logged and
// treated as "view unavailable" so the freezer stays on the safe degraded
// path rather than failing the block.
func voterWeightsFromSM(sm protocol.StateManager) VoterWeightView {
	v, err := sm.ReadView(_protocolID)
	if err != nil {
		return nil
	}
	vd, ok := v.(*viewData)
	if !ok || vd == nil {
		log.L().Warn("staking: view has unexpected type; poll snapshot Entries will be empty",
			zap.String("protocol", _protocolID))
		return nil
	}
	return vd.voterWeights
}

// TestOnlyPutPollSnapshotFor seeds a CandidatePollSnapshot directly under
// the same key layout FreezePollSnapshot uses. Intended solely for
// rewarding-package unit tests that exercise post-fork branches without
// standing up the full poll layer + DelegateProfile bridge. Production
// code MUST use FreezePollSnapshot at PutPollResult.
func TestOnlyPutPollSnapshotFor(
	sm protocol.StateManager,
	candID address.Address,
	snap *CandidatePollSnapshot,
) error {
	if candID == nil {
		return errors.New("staking: nil candidate identity")
	}
	if snap == nil {
		return errors.New("staking: nil snapshot")
	}
	_, err := sm.PutState(
		snap.toBlob(),
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(candidatePollSnapshotKey(candID)),
	)
	return err
}

// PollSnapshotFor returns the frozen snapshot written at the most recent
// PutPollResult for the given candidate identity. Returns
// (nil, state.ErrStateNotExist) when no snapshot has been written.
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

// CandidateRewardAddress returns the effective IIP-59 reward address and
// whether it was explicitly configured after activation. Migrated candidates
// default to their current owner; a post-fork register or reward-address update
// opts into the persisted Reward value.
func CandidateRewardAddress(sr protocol.StateReader, candID address.Address) (address.Address, bool, error) {
	var c Candidate
	if _, err := sr.State(
		&c,
		protocol.NamespaceOption(_candidateNameSpace),
		protocol.KeyOption(candID.Bytes()),
	); err != nil {
		return nil, false, err
	}
	if c.RewardAddressUpdated {
		return c.Reward, true, nil
	}
	return c.Owner, false, nil
}

// TestOnlyPutCandidateRewardAddress seeds persistent candidate state used by
// CandidateRewardAddress without requiring a full staking protocol fixture.
func TestOnlyPutCandidateRewardAddress(
	sm protocol.StateManager,
	candID address.Address,
	owner address.Address,
	reward address.Address,
	updated bool,
) error {
	if reward == nil {
		reward = owner
	}
	candidate := &Candidate{
		Owner: owner, Operator: owner, Reward: reward, Identifier: candID,
		Name: "iip59-owner", Votes: new(big.Int), SelfStake: new(big.Int),
		RewardAddressUpdated: updated,
	}
	if address.Equal(candID, owner) {
		candidate.Identifier = nil
	}
	_, err := sm.PutState(
		candidate,
		protocol.NamespaceOption(_candidateNameSpace),
		protocol.KeyOption(candID.Bytes()),
	)
	return err
}
