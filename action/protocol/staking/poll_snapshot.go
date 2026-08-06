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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/delegateprofile"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/distributedlog"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// _fullCommissionBasisPoints is 100% in basis points — the commission an
// opted-in candidate takes when DelegateProfile has no registered split for
// it, which reproduces the pre-IIP-59 behaviour of paying the delegate the
// whole amount.
const _fullCommissionBasisPoints uint64 = 10_000

// CandidatePollSnapshot is the frozen per-candidate view that IIP-59's
// rewarding path consumes at each epoch close. It is written once per reward
// era by FreezePollSnapshot (called from the poll layer's PutPollResult) for
// each opted-in candidate, and never mutated during the era. Mid-era
// DelegateProfile changes do not retroactively re-split rewards that have
// already begun accruing.
//
// Every field is a scalar, deliberately. The snapshot used to also carry the
// delegate's full materialized (voter, weight) list, which made the era
// boundary cost proportional to the voter population and duplicated the voter
// set into consensus state. The drain is voter-major now: it walks the voter
// key space and recomputes each weight from the era's copy-on-write bucket
// window, so the only thing it needs frozen per delegate is the denominator
// and the two inputs the recompute is sensitive to (FreezeHeight,
// SelfStakeBucketIdx).
type CandidatePollSnapshot struct {
	// BlockCommissionBasisPoints is the delegate's take of block rewards, in
	// basis points [0, 10000]. Defaults to 10000 when Registered is false.
	BlockCommissionBasisPoints uint64
	// EpochCommissionBasisPoints is the delegate's take of epoch rewards, in
	// basis points [0, 10000]. Defaults to 10000 when Registered is false.
	EpochCommissionBasisPoints uint64
	// Registered is true when the DelegateProfile contract returned both
	// portion fields as non-empty bytes at snapshot time.
	Registered bool
	// OnchainRewardEnabled freezes whether this candidate uses protocol-native
	// reward distribution during the current reward era.
	OnchainRewardEnabled bool
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
	// SnapshotHash is the deterministic digest of this delegate's frozen era
	// parameters. See eraSnapshotHash: it is the join key that lets off-chain
	// consumers assemble the partial DelegateDistributed logs one settlement
	// emits across many blocks.
	SnapshotHash hash.Hash256
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

// toBlob returns the wire form for PutState. Unlike the entry-list version it
// replaces, it derives nothing: TotalWeight and SnapshotHash are both frozen by
// FreezePollSnapshot, which is the only place that has the candidate record and
// the era height they are computed from.
func (s *CandidatePollSnapshot) toBlob() *candidatePollSnapshotBlob {
	return &candidatePollSnapshotBlob{pb: &stakingpb.CandidatePollSnapshot{
		BlockCommissionBasisPoints: s.BlockCommissionBasisPoints,
		EpochCommissionBasisPoints: s.EpochCommissionBasisPoints,
		Registered:                 s.Registered,
		OnchainRewardEnabled:       s.OnchainRewardEnabled,
		TotalWeight:                safeBigInt(s.TotalWeight).Bytes(),
		SnapshotHash:               s.SnapshotHash[:],
		FreezeHeight:               s.FreezeHeight,
		SelfStakeBucketIdx:         s.SelfStakeBucketIdx,
	}}
}

// fromBlob converts a decoded blob back into a CandidatePollSnapshot.
func fromBlob(b *candidatePollSnapshotBlob) (*CandidatePollSnapshot, error) {
	if b == nil || b.pb == nil {
		return &CandidatePollSnapshot{}, nil
	}
	return &CandidatePollSnapshot{
		BlockCommissionBasisPoints: b.pb.GetBlockCommissionBasisPoints(),
		EpochCommissionBasisPoints: b.pb.GetEpochCommissionBasisPoints(),
		Registered:                 b.pb.GetRegistered(),
		OnchainRewardEnabled:       b.pb.GetOnchainRewardEnabled(),
		TotalWeight:                new(big.Int).SetBytes(b.pb.GetTotalWeight()),
		SnapshotHash:               hash.BytesToHash256(b.pb.GetSnapshotHash()),
		FreezeHeight:               b.pb.GetFreezeHeight(),
		SelfStakeBucketIdx:         b.pb.GetSelfStakeBucketIdx(),
	}, nil
}

// safeBigInt returns v, or a fresh zero when v is nil.
func safeBigInt(v *big.Int) *big.Int {
	if v == nil {
		return new(big.Int)
	}
	return v
}

// eraSnapshotHash is the deterministic digest stamped into every
// DelegateDistributed log a settlement emits for this delegate.
//
// Its consumer is off-chain: one settlement pays a delegate's voters across
// many blocks, so the delegate's payout is reported as a stream of partial logs
// that a consumer has to reassemble. The reassembly key is
// (snapshotHash, delegate, epoch). What the hash therefore has to be is a
// stable per-delegate-per-era identifier -- constant for every chunk of one
// settlement, different for the next era -- and, ideally, a commitment to the
// parameters that determined the payouts being reassembled, so a consumer can
// recompute it from a snapshot read and confirm the logs it collected describe
// the era it thinks they do.
//
// It commits to exactly that: the candidate identifier plus every scalar the
// snapshot freezes. FreezeHeight makes it era-unique even for a delegate whose
// stake and commission did not move between two boundaries. It deliberately
// does not commit to the voter set: the voter set is no longer frozen, it is
// recomputed from the copy-on-write bucket window, and TotalWeight (the frozen
// candidate.Votes) is the aggregate that actually governs every share.
func eraSnapshotHash(candID address.Address, s *CandidatePollSnapshot) hash.Hash256 {
	if s == nil {
		return hash.ZeroHash256
	}
	return distributedlog.EraSnapshotHash(distributedlog.EraSnapshotParams{
		Delegate:                   candID,
		FreezeHeight:               s.FreezeHeight,
		TotalWeight:                safeBigInt(s.TotalWeight),
		SelfStakeBucketIdx:         s.SelfStakeBucketIdx,
		BlockCommissionBasisPoints: s.BlockCommissionBasisPoints,
		EpochCommissionBasisPoints: s.EpochCommissionBasisPoints,
		Registered:                 s.Registered,
		OnchainRewardEnabled:       s.OnchainRewardEnabled,
	})
}

// FreezePollSnapshot writes a CandidatePollSnapshot for every candidate that is
// on the IIP-59 rails at the era boundary, freezing state as of H, the height of
// the block it runs in. This is the *only* writer of the snapshot; rewarding is
// a pure reader via PollSnapshotFor.
//
// THE SET IS THE OPTED-IN CANDIDATE SET. It is enumerated from the candidate
// center and filtered by live routing: a candidate is frozen iff it is already
// using a configured Hermes vault, or has explicitly opted in.
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
// The converse costs nothing: an opted-out candidate gets no record at all.
// That is the same thing rewarding reads for a candidate it has never seen
// (voter_reward.go maps ErrStateNotExist to onchainRewardEnabled=false), so
// the placeholders this used to write for opted-out poll members were
// indistinguishable from their own absence.
//
// Sorted by identifier bytes, because the candidate center enumerates from a Go
// map and the order reaches both PutState and the DelegateProfile bridge call.
//
// A per-delegate bridge read failure (malformed profile, RPC/EVM error) is
// absorbed by the bridge itself: the affected delegate lands with
// Registered=false and rewarding uses the all-to-owner default. This
// prevents one bad on-chain profile from deterministically halting the
// chain at every epoch boundary — same state ⇒ same fallback on every
// validator ⇒ no fork.
//
// Note what is deliberately absent: any materialized per-voter weight list.
// The retired VoterWeightView had one, and freezing it meant the boundary
// had to degrade whenever the list was incomplete. TotalWeight now comes
// from the candidate record's own Votes accumulator, which is complete at
// every height, and the drain enumerates voters from the bucket indexes.
func FreezePollSnapshot(
	ctx context.Context,
	sm protocol.StateManager,
	bridge *delegateprofile.Bridge,
	reader delegateprofile.ContractReader,
) error {
	if bridge != nil && reader == nil {
		return errors.New("staking: nil ContractReader with non-nil DelegateProfile bridge")
	}
	g, _ := genesis.ExtractGenesisContext(ctx)

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
		return errors.Wrap(err, "staking: construct candidate view for poll snapshot")
	}
	if v := csr.BaseView(); v == nil || v.candCenter == nil {
		return errors.New("staking: no candidate center to freeze the poll snapshot from")
	}

	// Opt-in is re-read from state rather than taken off the center record in
	// hand. The two agree -- Candidate.Clone carries every field
	// candidateOnchainRewardEnabled looks at -- but ReadCandidateRewardRouting
	// is what every other reader of this bit uses, and one source for one
	// question is worth a read per candidate once per era.
	all := csr.AllCandidates()
	frozen := make([]*Candidate, 0, len(all))
	for _, c := range all {
		if c == nil {
			continue
		}
		id := c.GetIdentifier()
		if id == nil {
			continue
		}
		routing, rErr := ReadCandidateRewardRouting(sm, id, g.HermesRewardVaultAddresses)
		if rErr != nil {
			if errors.Is(rErr, state.ErrStateNotExist) {
				continue
			}
			return errors.Wrapf(rErr, "staking: read reward routing for candidate %s", id.String())
		}
		if routing.OnchainRewardEnabled {
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
			return errors.Wrap(err, "staking: DelegateProfile snapshot failed")
		}
	}

	freezeHeight, err := freezeHeightOf(ctx, sm)
	if err != nil {
		return err
	}
	// Unconditional, and not folded into the loop: the window's lifecycle is
	// keyed on "this epoch was an era boundary", not on the boundary having
	// found anything to freeze, and the zero-work seal in rewarding is what
	// closes an era that froze nothing.
	if err := beginEraCOWWindow(ctx, sm, freezeHeight); err != nil {
		return err
	}

	for _, cand := range frozen {
		id := cand.GetIdentifier()
		snap := &CandidatePollSnapshot{
			OnchainRewardEnabled:       true,
			FreezeHeight:               freezeHeight,
			SelfStakeBucketIdx:         cand.SelfStakeBucketIdx,
			TotalWeight:                new(big.Int),
			BlockCommissionBasisPoints: _fullCommissionBasisPoints,
			EpochCommissionBasisPoints: _fullCommissionBasisPoints,
		}
		if r, ok := rates[id.String()]; ok && r != nil && r.Registered {
			snap.BlockCommissionBasisPoints = r.BlockCommissionBasisPoints
			snap.EpochCommissionBasisPoints = r.EpochCommissionBasisPoints
			snap.Registered = true
		}
		// Copied, not aliased: the candidate center hands back a record whose
		// Votes keeps moving for the rest of the era.
		if cand.Votes != nil && cand.Votes.Sign() > 0 {
			snap.TotalWeight = new(big.Int).Set(cand.Votes)
		}
		// Last, so the digest covers the finished record.
		snap.SnapshotHash = eraSnapshotHash(id, snap)
		if err := writeCandidatePollSnapshot(sm, id, snap); err != nil {
			return err
		}
	}
	return nil
}

func writeCandidatePollSnapshot(
	sm protocol.StateManager,
	candID address.Address,
	snap *CandidatePollSnapshot,
) error {
	if _, err := sm.PutState(
		snap.toBlob(),
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(candidatePollSnapshotKey(candID)),
	); err != nil {
		return errors.Wrapf(err, "staking: write poll snapshot for candidate %s", candID.String())
	}
	return nil
}

// TestOnlyPutPollSnapshotFor seeds a CandidatePollSnapshot directly under
// the same key layout FreezePollSnapshot uses. Intended solely for
// rewarding-package unit tests that exercise post-fork branches without
// standing up the full poll layer + DelegateProfile bridge. Production
// code MUST use FreezePollSnapshot at PutPollResult.
//
// A zero SnapshotHash is filled in the same way the freezer would, so a test
// fixture cannot accidentally assert against a digest production would never
// have written. Set it explicitly to override.
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
	if snap.SnapshotHash == hash.ZeroHash256 {
		snap.SnapshotHash = eraSnapshotHash(candID, snap)
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

// CandidateRewardRouting is the deterministic IIP-59 routing view for a candidate.
type CandidateRewardRouting struct {
	Owner                address.Address
	LegacyRewardAddress  address.Address
	OnchainRewardEnabled bool
	ExplicitlyEnabled    bool
	RewardAddressUpdated bool
}

func ReadCandidateRewardRouting(
	sr protocol.StateReader,
	candID address.Address,
	hermesVaults []string,
) (*CandidateRewardRouting, error) {
	var c Candidate
	if _, err := sr.State(
		&c,
		protocol.NamespaceOption(_candidateNameSpace),
		protocol.KeyOption(candID.Bytes()),
	); err != nil {
		return nil, err
	}
	return &CandidateRewardRouting{
		Owner:                c.Owner,
		LegacyRewardAddress:  c.Reward,
		OnchainRewardEnabled: candidateOnchainRewardEnabled(&c, hermesVaults),
		ExplicitlyEnabled:    c.VoterRewardOnchainOptIn,
		RewardAddressUpdated: c.RewardAddressUpdated,
	}, nil
}

func candidateOnchainRewardEnabled(c *Candidate, hermesVaults []string) bool {
	if c == nil {
		return false
	}
	if c.VoterRewardOnchainOptIn {
		return true
	}
	if c.RewardAddressUpdated || c.Reward == nil {
		return false
	}
	for _, vault := range hermesVaults {
		if c.Reward.String() == vault {
			return true
		}
	}
	return false
}

// CandidateRewardAddress is retained for ReadState compatibility. It returns
// the persisted legacy reward address and whether it was updated post-fork.
func CandidateRewardAddress(sr protocol.StateReader, candID address.Address) (address.Address, bool, error) {
	routing, err := ReadCandidateRewardRouting(sr, candID, nil)
	if err != nil {
		return nil, false, err
	}
	if routing.RewardAddressUpdated {
		return routing.LegacyRewardAddress, true, nil
	}
	return routing.Owner, false, nil
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
		SelfStakeBucketIdx:   candidateNoSelfStakeBucketIndex,
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
