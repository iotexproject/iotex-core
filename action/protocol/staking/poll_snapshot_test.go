// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"encoding/binary"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/delegateprofile"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// The DelegateProfile ABI used by fakeStore.reader below is the same subset
// the bridge itself packs against, so a round-trip here exercises the same
// code path that consortium-style contract reads take in production.
const delegateProfileABI = `[
  {
    "inputs": [
      {"internalType": "address", "name": "_delegate", "type": "address"},
      {"internalType": "string", "name": "_field", "type": "string"}
    ],
    "name": "getProfileByField",
    "outputs": [{"internalType": "bytes", "name": "", "type": "bytes"}],
    "stateMutability": "view",
    "type": "function"
  }
]`

func TestCandidatePollSnapshot_SerializeRoundtrip(t *testing.T) {
	r := require.New(t)
	orig := &CandidatePollSnapshot{
		BlockCommissionBasisPoints: 1234,
		EpochCommissionBasisPoints: 5678,
		Registered:                 true,
		OnchainRewardEnabled:       true,
		TotalWeight:                big.NewInt(3_500_000),
		SnapshotHash:               hash.Hash256b([]byte("era-42")),
		FreezeHeight:               909_090,
		SelfStakeBucketIdx:         7,
	}
	blob := orig.toBlob()
	buf, err := blob.Serialize()
	r.NoError(err)

	var round candidatePollSnapshotBlob
	r.NoError(round.Deserialize(buf))
	out, err := fromBlob(&round)
	r.NoError(err)

	r.Equal(orig.BlockCommissionBasisPoints, out.BlockCommissionBasisPoints)
	r.Equal(orig.EpochCommissionBasisPoints, out.EpochCommissionBasisPoints)
	r.Equal(orig.Registered, out.Registered)
	r.Equal(orig.OnchainRewardEnabled, out.OnchainRewardEnabled)
	r.Zero(orig.TotalWeight.Cmp(out.TotalWeight))
	r.Equal(orig.SnapshotHash, out.SnapshotHash)
	r.Equal(orig.FreezeHeight, out.FreezeHeight)
	r.Equal(orig.SelfStakeBucketIdx, out.SelfStakeBucketIdx)
}

func TestCandidatePollSnapshot_SerializeZeroTotalWeight(t *testing.T) {
	// A snapshot with no payable voter set (opted-out delegate, or a candidate
	// record the freezer could not read) must round-trip to a zero, non-nil
	// TotalWeight — that is the value rewarding tests with Sign() to decide
	// whether the delegate has anything to distribute this era.
	r := require.New(t)
	orig := &CandidatePollSnapshot{
		BlockCommissionBasisPoints: 1000,
		EpochCommissionBasisPoints: 2000,
		Registered:                 true,
	}
	blob := orig.toBlob()
	buf, err := blob.Serialize()
	r.NoError(err)

	var round candidatePollSnapshotBlob
	r.NoError(round.Deserialize(buf))
	out, err := fromBlob(&round)
	r.NoError(err)
	r.NotNil(out.TotalWeight)
	r.Zero(out.TotalWeight.Sign())
	r.True(out.Registered)
}

func TestCandidatePollSnapshot_ZeroValueSerializes(t *testing.T) {
	// A zero-value CandidatePollSnapshot must round-trip cleanly — this is
	// what a delegate looks like when the DelegateProfile bridge is
	// disabled.
	r := require.New(t)
	orig := &CandidatePollSnapshot{}
	blob := orig.toBlob()
	buf, err := blob.Serialize()
	r.NoError(err)

	var pb stakingpb.CandidatePollSnapshot
	r.NoError(proto.Unmarshal(buf, &pb))
	r.Zero(pb.GetBlockCommissionBasisPoints())
	r.Zero(pb.GetEpochCommissionBasisPoints())
	r.False(pb.GetRegistered())
	r.Empty(pb.GetTotalWeight())
	r.Zero(pb.GetFreezeHeight())
}

func TestCandidatePollSnapshotKey_Layout(t *testing.T) {
	r := require.New(t)
	// Key is exactly 1 tag byte + candidate address bytes; tag byte is the
	// _candidatePollSnapshot constant. This test guards against accidental
	// reuse of another byte for the same key layout.
	candID := identityset.Address(3)
	key := candidatePollSnapshotKey(candID)
	r.Equal(1+len(candID.Bytes()), len(key))
	r.Equal(_candidatePollSnapshot, key[0])
	r.Equal(candID.Bytes(), key[1:])
}

func TestCandidateRewardAddress(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(2), Reward: identityset.Address(3),
		Name: "delegate", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	r.NoError(putOnchainCandidate(csm, cand))
	got, explicitlySet, err := CandidateRewardAddress(sm, cand.GetIdentifier())
	r.NoError(err)
	r.True(address.Equal(cand.Owner, got))
	r.False(explicitlySet)

	cand.Owner = identityset.Address(7)
	r.NoError(putOnchainCandidate(csm, cand))
	got, explicitlySet, err = CandidateRewardAddress(sm, cand.GetIdentifier())
	r.NoError(err)
	r.True(address.Equal(cand.Owner, got), "default reward address must follow owner transfers")
	r.False(explicitlySet)

	cand.RewardAddressUpdated = true
	r.NoError(putOnchainCandidate(csm, cand))
	got, explicitlySet, err = CandidateRewardAddress(sm, cand.GetIdentifier())
	r.NoError(err)
	r.True(address.Equal(cand.Reward, got))
	r.True(explicitlySet)

	_, _, err = CandidateRewardAddress(sm, identityset.Address(9))
	r.ErrorIs(err, state.ErrStateNotExist)
}

func TestCandidateRewardRoutingModes(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	vault, err := address.FromString("io19604a05s2p3mecam2zz7d27hcr6ndyw80wvkmh")
	r.NoError(err)
	other := identityset.Address(3)
	candidate := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(2), Reward: vault,
		Name: "delegate", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	r.NoError(csm.putCandidate(candidate))

	routing, err := ReadCandidateRewardRouting(sm, candidate.GetIdentifier(), []string{vault.String()})
	r.NoError(err)
	r.True(routing.OnchainRewardEnabled)
	r.True(address.Equal(candidate.Owner, routing.Owner))
	r.True(address.Equal(vault, routing.LegacyRewardAddress))

	candidate.Reward = other
	r.NoError(csm.putCandidate(candidate))
	routing, err = ReadCandidateRewardRouting(sm, candidate.GetIdentifier(), []string{vault.String()})
	r.NoError(err)
	r.False(routing.OnchainRewardEnabled)

	candidate.Reward = vault
	candidate.RewardAddressUpdated = true
	r.NoError(csm.putCandidate(candidate))
	routing, err = ReadCandidateRewardRouting(sm, candidate.GetIdentifier(), []string{vault.String()})
	r.NoError(err)
	r.False(routing.OnchainRewardEnabled, "post-fork address updates must not trigger automatic migration")

	candidate.VoterRewardOnchainOptIn = true
	r.NoError(csm.putCandidate(candidate))
	routing, err = ReadCandidateRewardRouting(sm, candidate.GetIdentifier(), []string{vault.String()})
	r.NoError(err)
	r.True(routing.OnchainRewardEnabled)
	r.True(routing.ExplicitlyEnabled)
}

// A legacy candidate -- one that pays its voters off-chain -- is enumerated by
// the freezer along with everyone else and then dropped on the opt-in test,
// before either the profile read or the snapshot write. Both omissions are
// asserted: the reader fails the test if called, and the candidate ends the
// boundary with no record at all.
func TestFreezePollSnapshot_LegacyCandidateSkipsProfileAndSnapshot(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	candidate := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(2), Reward: identityset.Address(3),
		Name: "legacy", Votes: big.NewInt(1), SelfStake: big.NewInt(1), RewardAddressUpdated: true,
	}
	r.NoError(csm.putCandidate(candidate))
	installCandCenter(t, sm, candidate)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)
	reader := delegateprofile.ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		t.Fatal("legacy candidate must not trigger a profile read")
		return nil, nil
	})
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	r.NoError(FreezePollSnapshot(ctx, sm, bridge, reader))

	_, err = PollSnapshotFor(sm, candidate.GetIdentifier())
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist)
}

func TestFreezePollSnapshot_NilBridge(t *testing.T) {
	// Bridge nil path: the snapshot writer still runs (post-fork, no
	// contract configured), records Registered=false and full owner commission.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	firstCand := &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(2),
		Reward:             identityset.Address(3),
		Name:               "first-delegate",
		Votes:              big.NewInt(1),
		SelfStake:          big.NewInt(1),
		SelfStakeBucketIdx: 1,
	}
	secondCand := &Candidate{
		Owner:              identityset.Address(4),
		Operator:           identityset.Address(5),
		Reward:             identityset.Address(6),
		Name:               "second-delegate",
		Votes:              big.NewInt(1),
		SelfStake:          big.NewInt(1),
		SelfStakeBucketIdx: 2,
	}
	r.NoError(putOnchainCandidate(csm, firstCand))
	r.NoError(putOnchainCandidate(csm, secondCand))
	installCandCenter(t, sm, firstCand, secondCand)

	r.NoError(FreezePollSnapshot(context.Background(), sm, nil, nil))

	snap, err := PollSnapshotFor(sm, firstCand.Owner)
	r.NoError(err)
	r.False(snap.Registered)
	r.Equal(uint64(10000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(10000), snap.EpochCommissionBasisPoints)
	r.True(snap.OnchainRewardEnabled)

	snap, err = PollSnapshotFor(sm, secondCand.Owner)
	r.NoError(err)
	r.False(snap.Registered)
}

func TestFreezePollSnapshot_HappyPath(t *testing.T) {
	// End-to-end path with a working bridge + reader: rates come back
	// registered=true with the expected inversion from voter-take portion
	// (9000, 8000) → commission (1000, 2000) basis points.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := &Candidate{
		Owner:     identityset.Address(1),
		Operator:  identityset.Address(2),
		Reward:    identityset.Address(3),
		Name:      "registered-delegate",
		Votes:     big.NewInt(1),
		SelfStake: big.NewInt(1),
	}
	r.NoError(putOnchainCandidate(csm, cand))
	installCandCenter(t, sm, cand)

	fake := newFakeProfileStore()
	fake.setPortion(cand.Owner, "blockRewardPortion", 9000)
	fake.setPortion(cand.Owner, "epochRewardPortion", 8000)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)

	r.NoError(FreezePollSnapshot(context.Background(), sm, bridge, fake.reader(t)))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.True(snap.Registered)
	r.Equal(uint64(1000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(2000), snap.EpochCommissionBasisPoints)
}

func TestFreezePollSnapshot_UsesCandidateIdentity(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	identity := identityset.Address(9)
	operator := identityset.Address(2)
	cand := &Candidate{
		Owner:      identityset.Address(1),
		Identifier: identity,
		Operator:   operator,
		Reward:     identityset.Address(3),
		Name:       "stable-identity",
		Votes:      big.NewInt(1),
		SelfStake:  big.NewInt(1),
	}
	r.NoError(putOnchainCandidate(csm, cand))
	installCandCenter(t, sm, cand)

	fake := newFakeProfileStore()
	fake.setPortion(identity, "blockRewardPortion", 9000)
	fake.setPortion(identity, "epochRewardPortion", 8000)
	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)

	r.NoError(FreezePollSnapshot(context.Background(), sm, bridge, fake.reader(t)))

	snapshot, err := PollSnapshotFor(sm, identity)
	r.NoError(err)
	r.True(snapshot.Registered)
	_, err = PollSnapshotFor(sm, operator)
	r.ErrorIs(err, state.ErrStateNotExist)
}

func TestFreezePollSnapshot_PartialProfile(t *testing.T) {
	// One delegate registered on-chain, another absent from DelegateProfile.
	// The unregistered one still gets a snapshot row (Registered=false),
	// and therefore uses the all-to-owner default.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	registered := &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(2),
		Reward:             identityset.Address(3),
		Name:               "registered",
		Votes:              big.NewInt(1),
		SelfStake:          big.NewInt(1),
		SelfStakeBucketIdx: 1,
	}
	unregistered := &Candidate{
		Owner:              identityset.Address(4),
		Operator:           identityset.Address(5),
		Reward:             identityset.Address(6),
		Name:               "unregistered",
		Votes:              big.NewInt(1),
		SelfStake:          big.NewInt(1),
		SelfStakeBucketIdx: 2,
	}
	r.NoError(putOnchainCandidate(csm, registered))
	r.NoError(putOnchainCandidate(csm, unregistered))
	installCandCenter(t, sm, registered, unregistered)

	fake := newFakeProfileStore()
	// Only `registered` has portion fields set.
	fake.setPortion(registered.Owner, "blockRewardPortion", 500)
	fake.setPortion(registered.Owner, "epochRewardPortion", 750)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)

	r.NoError(FreezePollSnapshot(context.Background(), sm, bridge, fake.reader(t)))

	snap, err := PollSnapshotFor(sm, registered.Owner)
	r.NoError(err)
	r.True(snap.Registered)
	r.Equal(uint64(9500), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(9250), snap.EpochCommissionBasisPoints)

	snap, err = PollSnapshotFor(sm, unregistered.Owner)
	r.NoError(err)
	r.False(snap.Registered)
	r.Equal(uint64(10000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(10000), snap.EpochCommissionBasisPoints)
}

func TestFreezePollSnapshot_BridgeErrorDegradesToLegacy(t *testing.T) {
	// A per-delegate bridge failure must NOT abort the block. Any single
	// delegate's read error deterministically halts the chain at every epoch
	// boundary — worse than the reward-misroute it would prevent. The
	// snapshot is still written with Registered=false and full owner commission.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := &Candidate{
		Owner:     identityset.Address(1),
		Operator:  identityset.Address(2),
		Reward:    identityset.Address(3),
		Name:      "delegate",
		Votes:     big.NewInt(1),
		SelfStake: big.NewInt(1),
	}
	r.NoError(putOnchainCandidate(csm, cand))
	installCandCenter(t, sm, cand)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)
	failing := delegateprofile.ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		return nil, errors.New("rpc down")
	})

	r.NoError(FreezePollSnapshot(context.Background(), sm, bridge, failing))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.False(snap.Registered)
	r.Equal(uint64(10000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(10000), snap.EpochCommissionBasisPoints)
}

// The old TestFreezePollSnapshot_InvalidCandidateAddress lived here. It
// asserted that an unparseable identity in the poll list aborted the freeze;
// that parse no longer exists, because identities now come from the candidate
// center as address.Address values that were parsed when the candidate was
// registered.

func TestFreezePollSnapshot_NilBridgeSkipsReader(t *testing.T) {
	// A reader with no bridge is not an error -- there is nothing to read
	// against -- but it must not be invoked. The empty candidate center makes
	// this the empty-era case too: a boundary that finds no opted-in candidate
	// still opens the window and succeeds.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	installCandCenter(t, sm)

	// Reader that would fail the test if called — proves nil-bridge path skips it.
	panicReader := delegateprofile.ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		t.Fatalf("reader must not be called when bridge is nil")
		return nil, nil
	})
	r.NoError(FreezePollSnapshot(context.Background(), sm, nil, panicReader))
}

func TestFreezePollSnapshot_NonNilBridgeRequiresReader(t *testing.T) {
	// The wiring check runs before the freezer touches state, so this case
	// needs no view: a bridge with no reader is a caller bug, not a state
	// condition.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)
	err = FreezePollSnapshot(context.Background(), sm, bridge, nil)
	r.Error(err)
	r.Contains(err.Error(), "nil ContractReader")
}

func TestPollSnapshotFor_MissingReturnsErrStateNotExist(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	_, err := PollSnapshotFor(sm, identityset.Address(1))
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist)
}

func TestPollSnapshotFor_NilCandidateRejected(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	_, err := PollSnapshotFor(sm, nil)
	r.Error(err)
}

// TestFreezePollSnapshot_EveryMemberGetsItsOwnRates is the multi-delegate
// bridge case: each member of the frozen set is looked up separately and each
// gets its own rates back, with no bleed between them. The order the bridge
// sees is pinned separately, by
// TestFreezePollSnapshot_FrozenSetOrderIsDeterministic.
func TestFreezePollSnapshot_EveryMemberGetsItsOwnRates(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	var cands []*Candidate
	for i := 1; i <= 3; i++ {
		c := &Candidate{
			Owner:              identityset.Address(i),
			Operator:           identityset.Address(i + 10),
			Reward:             identityset.Address(i + 20),
			Name:               "d" + string(rune('0'+i)),
			Votes:              big.NewInt(1),
			SelfStake:          big.NewInt(1),
			SelfStakeBucketIdx: uint64(i),
		}
		r.NoError(putOnchainCandidate(csm, c))
		cands = append(cands, c)
	}
	installCandCenter(t, sm, cands...)

	fake := newFakeProfileStore()
	for i, c := range cands {
		// Distinct per delegate, so a cross-assignment shows up as a wrong
		// number rather than as an identical one.
		fake.setPortion(c.Owner, "blockRewardPortion", uint64(4000+i*100))
		fake.setPortion(c.Owner, "epochRewardPortion", uint64(5000+i*100))
	}
	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)

	r.NoError(FreezePollSnapshot(context.Background(), sm, bridge, fake.reader(t)))
	for i, c := range cands {
		snap, err := PollSnapshotFor(sm, c.Owner)
		r.NoError(err)
		r.True(snap.Registered)
		r.Equal(uint64(6000-i*100), snap.BlockCommissionBasisPoints)
		r.Equal(uint64(5000-i*100), snap.EpochCommissionBasisPoints)
	}
}

// ---------------------------------------------------------------------------
// Test helpers — a tiny fake DelegateProfile ABI backend.
// Mirrors the fakeStore pattern in delegateprofile/bridge_test.go so the same
// call packing/unpacking exercises the bridge inside the staking-side test.
// ---------------------------------------------------------------------------

type fakeProfileStore struct {
	values map[string][]byte
}

func putOnchainCandidate(csm CandidateStateManager, candidate *Candidate) error {
	candidate.VoterRewardOnchainOptIn = true
	return csm.putCandidate(candidate)
}

func newFakeProfileStore() *fakeProfileStore {
	return &fakeProfileStore{values: map[string][]byte{}}
}

func (s *fakeProfileStore) setPortion(delegate address.Address, field string, bp uint64) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, bp)
	trimmed := buf
	for len(trimmed) > 0 && trimmed[0] == 0 {
		trimmed = trimmed[1:]
	}
	s.values[storeKey(delegate, field)] = trimmed
}

func storeKey(delegate address.Address, field string) string {
	return common.BytesToAddress(delegate.Bytes()).Hex() + "|" + field
}

func (s *fakeProfileStore) reader(t *testing.T) delegateprofile.ContractReader {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(delegateProfileABI))
	require.NoError(t, err)
	return delegateprofile.ContractReaderFunc(func(_ context.Context, contract string, callData []byte) ([]byte, error) {
		if contract == "" {
			return nil, errors.New("empty contract")
		}
		if len(callData) < 4 {
			return nil, errors.New("truncated call data")
		}
		method, err := parsed.MethodById(callData[:4])
		if err != nil {
			return nil, err
		}
		args, err := method.Inputs.Unpack(callData[4:])
		if err != nil {
			return nil, err
		}
		delegateEth := args[0].(common.Address)
		field := args[1].(string)
		addr, err := address.FromBytes(delegateEth.Bytes())
		if err != nil {
			return nil, err
		}
		return method.Outputs.Pack(s.values[storeKey(addr, field)])
	})
}

// Prevent unused-import warnings when the test file evolves during rebases.
var _ = protocol.StateManager(nil)
