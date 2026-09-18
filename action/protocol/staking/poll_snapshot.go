// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/big"
	"sort"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/delegateprofile"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/freezelog"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// _fullCommissionBasisPoints is 100% in basis points — the commission an
// opted-in candidate takes when DelegateProfile has no registered split for
// it, which reproduces the pre-IIP-59 behaviour of paying the delegate the
// whole amount.
const _fullCommissionBasisPoints uint64 = 10_000

// CandidateRewardSnapshot is the frozen per-candidate view that IIP-59's
// rewarding path consumes at each epoch close. It is written once per reward
// era by FreezeCandidateRewardSnapshots (called from the poll layer's PutPollResult) for
// each opted-in candidate, and never mutated during the era. Mid-era
// DelegateProfile changes do not retroactively re-split rewards that have
// already begun accruing.
//
// Every field is a scalar, deliberately. The snapshot used to also carry the
// delegate's full materialized (voter, weight) list, which made the era
// boundary cost proportional to the voter population and duplicated the voter
// set into consensus state. The drain is voter-major now: it walks the voter
// key space and recomputes each weight from the era's copy-on-write bucket
// window. Besides the commission policy, it only needs the denominator and
// the two inputs the recompute is sensitive to (FreezeHeight,
// SelfStakeBucketIdx).
type CandidateRewardSnapshot struct {
	// BlockCommissionBasisPoints is the delegate's take of block rewards, in
	// basis points [0, 10000]. Defaults to 10000 when CommissionConfigured is false.
	BlockCommissionBasisPoints uint64
	// EpochCommissionBasisPoints is the delegate's take of epoch rewards, in
	// basis points [0, 10000]. Defaults to 10000 when CommissionConfigured is false.
	EpochCommissionBasisPoints uint64
	// CommissionConfigured is true when DelegateProfile returned both reward
	// portion fields as non-empty, valid values at snapshot time. It is not the
	// result of DelegateProfile.registered(address).
	CommissionConfigured bool
	// TotalWeight is the denominator the drain divides each recomputed voter
	// weight by: the frozen value of the candidate's Votes accumulator at H.
	//
	// candidate.Votes is the accepted denominator because it is the same
	// number the removed entry list summed to -- TestVoterWeightInvariant
	// asserts candidate.Votes == Σ_voters view[cand][voter] after every
	// staking handler -- read from the one place that still exists at H.
	// Zero means "this era has no payable voter set for this delegate"; the
	// delegate's pending pool is left intact and rolls into a later era.
	TotalWeight *big.Int
	// FreezeHeight is the era boundary height H this snapshot was taken at.
	//
	// The IIP-59 drain runs several blocks after H and recomputes voter weights
	// from bucket state. Contract-staking buckets that are not timestamp-based
	// have their remaining duration measured against a block height, so the
	// recompute has to be handed H rather than the height of whichever block
	// the chunk runs in. Copy-on-write cannot fix that on its own -- the input
	// is the evaluation height, not a stored value -- so H travels with the
	// snapshot.
	FreezeHeight uint64
	// SelfStakeBucketIdx is the candidate's self-stake bucket index at H.
	//
	// This is the only field of the candidate record the weight recompute
	// reads (`isSelfStake := b.ContractAddress == "" && b.Index ==
	// cand.SelfStakeBucketIdx`), so it is frozen as a scalar rather than
	// copy-on-writing the whole candidate record and the endorsement keys the
	// live lookup goes through. candidateNoSelfStakeBucketIndex
	// (math.MaxUint64) means "no self-stake bucket".
	SelfStakeBucketIdx uint64
}

// Serialize implements state.Serializer.
func (s *CandidateRewardSnapshot) Serialize() ([]byte, error) {
	if s == nil {
		return proto.Marshal(&stakingpb.CandidateRewardSnapshot{})
	}
	return proto.Marshal(&stakingpb.CandidateRewardSnapshot{
		BlockCommissionBasisPoints: s.BlockCommissionBasisPoints,
		EpochCommissionBasisPoints: s.EpochCommissionBasisPoints,
		CommissionConfigured:       s.CommissionConfigured,
		TotalWeight:                safeBigInt(s.TotalWeight).Bytes(),
		FreezeHeight:               s.FreezeHeight,
		SelfStakeBucketIdx:         s.SelfStakeBucketIdx,
	})
}

// Deserialize implements state.Deserializer.
func (s *CandidateRewardSnapshot) Deserialize(buf []byte) error {
	pb := &stakingpb.CandidateRewardSnapshot{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate reward snapshot")
	}
	s.BlockCommissionBasisPoints = pb.GetBlockCommissionBasisPoints()
	s.EpochCommissionBasisPoints = pb.GetEpochCommissionBasisPoints()
	s.CommissionConfigured = pb.GetCommissionConfigured()
	s.TotalWeight = new(big.Int).SetBytes(pb.GetTotalWeight())
	s.FreezeHeight = pb.GetFreezeHeight()
	s.SelfStakeBucketIdx = pb.GetSelfStakeBucketIdx()
	return nil
}

// Encode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (s *CandidateRewardSnapshot) Encode() (systemcontracts.GenericValue, error) {
	data, err := s.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode implements systemcontracts.GenericValueContainer for Erigon dual-storage.
func (s *CandidateRewardSnapshot) Decode(v systemcontracts.GenericValue) error {
	return s.Deserialize(v.PrimaryData)
}

// candidateRewardSnapshotKey returns the state-trie key for a candidate's
// frozen reward snapshot: {_candidateRewardSnapshot} || candID.Bytes(). Namespace
// is _stakingNameSpace (see protocol.go). Mirrors the layout of the other
// candidate-scoped keys in this package.
func candidateRewardSnapshotKey(candID address.Address) []byte {
	out := make([]byte, 1, 1+len(candID.Bytes()))
	out[0] = _candidateRewardSnapshot
	return append(out, candID.Bytes()...)
}

// safeBigInt returns v, or a fresh zero when v is nil.
func safeBigInt(v *big.Int) *big.Int {
	if v == nil {
		return new(big.Int)
	}
	return v
}

// FreezeCandidateRewardSnapshots writes a CandidateRewardSnapshot for every candidate that is
// on the IIP-59 rails at freeze block H. This is the *only* writer of the snapshot; rewarding is
// a pure reader via CandidateRewardSnapshotFor.
//
// THE SET IS THE OPTED-IN CANDIDATE SET. It is enumerated from the candidate
// center and filtered by the persisted VoterRewardOnchainOptIn bit. The
// activation migration sets that bit for pre-IIP-59 Hermes candidates.
//
// It deliberately has nothing to do with the poll list this used to be handed.
// That list is filtered twice before a PutPollResult carries it -- ActiveCandidates
// drops anything failing isActiveCandidate, filterAndSortCandidatesByVoteScore
// drops anything below the vote-score threshold -- while this runs once per
// reward era (EpochsPerRewardEra epochs, ~24h on mainnet) and the set that
// actually receives epoch rewards is recomputed by rewarding at EVERY epoch
// inside that era. The two drift, and a candidate that is opted in but not
// frozen loses its voters a whole era: every reader treats "no snapshot" as
// "not on the rails", so the commission split falls back to 100% delegate /
// 0% voter, silently, for up to a full day. Freezing from the opt-in set
// closes that by construction rather than by union.
//
// A candidate that has not opted in gets no record. Rewarding treats snapshot
// absence as the legacy route for this era, so no explicit disabled record is
// needed.
//
// Sorted by identifier bytes, because the candidate center enumerates from a Go
// map and the order reaches both PutState and the DelegateProfile bridge call.
//
// A per-delegate bridge read failure is
// absorbed by the bridge itself: the affected delegate lands with
// CommissionConfigured=false and rewarding uses the all-to-owner default. This
// prevents one bad on-chain profile from halting every era boundary.
//
// Note what is deliberately absent: any materialized per-voter weight list.
// The retired VoterWeightView had one, and freezing it meant the boundary
// had to degrade whenever the list was incomplete. TotalWeight now comes
// from the candidate record's own Votes accumulator, which is complete at
// every height, and the drain enumerates voters from the bucket indexes.
func FreezeCandidateRewardSnapshots(
	ctx context.Context,
	sm protocol.StateManager,
	bridge *delegateprofile.Bridge,
	reader delegateprofile.ContractReader,
	freezeHeight uint64,
	era uint64,
) ([]*action.Log, error) {
	if bridge != nil && reader == nil {
		return nil, errors.New("staking: nil ContractReader with non-nil DelegateProfile bridge")
	}
	// The candidate center is the sole source of the frozen set, of
	// SelfStakeBucketIdx, and of TotalWeight, so failing to reach it is
	// returned rather than degraded -- including the protocol.ErrNoName and
	// nil-candCenter shapes, which an earlier version of this function
	// tolerated back when the poll list could still supply a set without it.
	//
	// This is the one place in IIP-59 where halting beats degrading. Freezing
	// an empty era would not degrade one item: it would put every delegate on
	// the 100%-commission fallback and pay no voter anything for the whole era,
	// identically and irrecoverably on every validator that saw the same fault.
	// A block that does not produce is recoverable; a frozen wrong era is not.
	//
	// Nothing is lost in production. Both shapes mean the staking protocol
	// installed no view, which is a property of the registry rather than of
	// chain data -- a pre-fork setup, or a test writing partial state directly.
	// Post-fork the view is installed by the staking protocol's own Start, and
	// the poll protocol that calls this holds a reference to that protocol.
	csr, err := ConstructBaseView(sm)
	if err != nil {
		return nil, errors.Wrap(err, "staking: construct candidate view for reward snapshots")
	}
	if v := csr.BaseView(); v == nil || v.candCenter == nil {
		return nil, errors.New("staking: no candidate center to freeze reward snapshots from")
	}

	all := csr.AllCandidates()
	frozen := make([]*Candidate, 0, len(all))
	for _, c := range all {
		if c != nil && c.GetIdentifier() != nil && c.VoterRewardOnchainOptIn {
			frozen = append(frozen, c)
		}
	}
	sort.Slice(frozen, func(i, j int) bool {
		return bytes.Compare(frozen[i].GetIdentifier().Bytes(), frozen[j].GetIdentifier().Bytes()) < 0
	})

	var rates map[string]*delegateprofile.CommissionRates
	if bridge != nil {
		ids := make([]address.Address, len(frozen))
		for i, c := range frozen {
			ids[i] = c.GetIdentifier()
		}
		rates, err = bridge.Snapshot(ctx, reader, ids)
		if err != nil {
			return nil, errors.Wrap(err, "staking: DelegateProfile snapshot failed")
		}
	}

	// Non-panicking: this function is called directly by tests with a bare
	// context, and "no feature context" can only mean "not gated on", never
	// "emit". Production always arrives through poll's handle, which builds one.
	fCtx, hasFeature := protocol.GetFeatureCtx(ctx)
	emitLogs := hasFeature && fCtx.EmitEraFreezeLog
	var (
		logs         []*action.Log
		protocolAddr string
		blockHeight  uint64
	)
	if emitLogs {
		protocolAddr = ProtocolAddr().String()
		blockHeight = protocol.MustGetBlockCtx(ctx).BlockHeight
	}

	for _, cand := range frozen {
		id := cand.GetIdentifier()
		snap := &CandidateRewardSnapshot{
			FreezeHeight:               freezeHeight,
			SelfStakeBucketIdx:         cand.SelfStakeBucketIdx,
			TotalWeight:                new(big.Int),
			BlockCommissionBasisPoints: _fullCommissionBasisPoints,
			EpochCommissionBasisPoints: _fullCommissionBasisPoints,
		}
		if r, ok := rates[id.String()]; ok && r != nil && r.Configured {
			snap.BlockCommissionBasisPoints = r.BlockCommissionBasisPoints
			snap.EpochCommissionBasisPoints = r.EpochCommissionBasisPoints
			snap.CommissionConfigured = true
		}
		// Copied, not aliased: the candidate center hands back a record whose
		// Votes keeps moving for the rest of the era.
		if cand.Votes != nil && cand.Votes.Sign() > 0 {
			snap.TotalWeight = new(big.Int).Set(cand.Votes)
		}
		if err := writeCandidateRewardSnapshot(sm, id, snap); err != nil {
			return nil, err
		}
		// Emitted inside this loop, which iterates `frozen` -- already sorted by
		// identifier bytes above. The candidate center enumerates from a Go map,
		// so without that ordering the log sequence would differ between nodes
		// and the receipt root with it.
		if emitLogs {
			topics, data, err := freezelog.Pack(freezelog.EventArgs{
				Era:                  era,
				Delegate:             id,
				FreezeHeight:         freezeHeight,
				BlockCommissionBps:   snap.BlockCommissionBasisPoints,
				EpochCommissionBps:   snap.EpochCommissionBasisPoints,
				CommissionConfigured: snap.CommissionConfigured,
				TotalWeight:          snap.TotalWeight,
				SelfStakeBucketIdx:   snap.SelfStakeBucketIdx,
			})
			if err != nil {
				return nil, errors.Wrapf(err, "staking: pack freeze log for candidate %s", id.String())
			}
			logs = append(logs, &action.Log{
				Address:     protocolAddr,
				Topics:      topics,
				Data:        data,
				BlockHeight: blockHeight,
			})
		}
	}
	return logs, nil
}

func writeCandidateRewardSnapshot(
	sm protocol.StateManager,
	candID address.Address,
	snap *CandidateRewardSnapshot,
) error {
	if _, err := sm.PutState(
		snap,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(candidateRewardSnapshotKey(candID)),
	); err != nil {
		return errors.Wrapf(err, "staking: write reward snapshot for candidate %s", candID.String())
	}
	return nil
}

// TestOnlyPutCandidateRewardSnapshotFor seeds a CandidateRewardSnapshot directly under
// the same key layout FreezeCandidateRewardSnapshots uses. Intended solely for
// rewarding-package unit tests that exercise post-fork branches without
// standing up the full poll layer + DelegateProfile bridge. Production
// code MUST use FreezeCandidateRewardSnapshots at PutPollResult.
func TestOnlyPutCandidateRewardSnapshotFor(
	sm protocol.StateManager,
	candID address.Address,
	snap *CandidateRewardSnapshot,
) error {
	if candID == nil {
		return errors.New("staking: nil candidate identity")
	}
	if snap == nil {
		return errors.New("staking: nil snapshot")
	}
	_, err := sm.PutState(
		snap,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(candidateRewardSnapshotKey(candID)),
	)
	return err
}

// CandidateRewardSnapshotFor returns the frozen snapshot written at the most recent
// PutPollResult for the given candidate identity. Returns
// (nil, state.ErrStateNotExist) when no snapshot has been written.
func CandidateRewardSnapshotFor(sr protocol.StateReader, candID address.Address) (*CandidateRewardSnapshot, error) {
	if candID == nil {
		return nil, errors.New("staking: nil candidate identity")
	}
	snapshot := &CandidateRewardSnapshot{}
	if _, err := sr.State(
		snapshot,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(candidateRewardSnapshotKey(candID)),
	); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// CandidateRewardAddress is retained for ReadState compatibility. It returns
// the persisted legacy reward address and whether it was updated post-fork.
func CandidateRewardAddress(sr protocol.StateReader, candID address.Address) (address.Address, bool, error) {
	candidate, _, err := NewCandidateByAddressReader(sr).CandidateByAddress(candID)
	if err != nil {
		return nil, false, err
	}
	if candidate.RewardAddressUpdated {
		return candidate.Reward, true, nil
	}
	return candidate.Owner, false, nil
}

// TestOnlyPutCandidateRewardAddress seeds candidate state used by rewarding
// tests. When a staking view exists it updates state through CandidateStateManager.
func TestOnlyPutCandidateRewardAddress(
	ctx context.Context,
	sm protocol.StateManager,
	candID address.Address,
	owner address.Address,
	reward address.Address,
	updated bool,
	optedIn bool,
) error {
	if reward == nil {
		reward = owner
	}
	candidate := &Candidate{
		Owner: owner, Operator: owner, Reward: reward, Identifier: candID,
		Name: hex.EncodeToString(candID.Bytes()[:6]), Votes: new(big.Int), SelfStake: new(big.Int),
		SelfStakeBucketIdx:      candidateNoSelfStakeBucketIndex,
		RewardAddressUpdated:    updated,
		VoterRewardOnchainOptIn: optedIn,
	}
	if address.Equal(candID, owner) {
		candidate.Identifier = nil
	}
	if csm, err := NewCandidateStateManagerWithContext(ctx, sm); err == nil {
		return csm.Upsert(candidate)
	}
	_, err := sm.PutState(
		candidate,
		protocol.NamespaceOption(_candidateNameSpace),
		protocol.KeyOption(candID.Bytes()),
	)
	return err
}
