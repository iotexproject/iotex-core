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

func TestCandidateRewardSnapshot_SerializeRoundtrip(t *testing.T) {
	r := require.New(t)
	orig := &CandidateRewardSnapshot{
		BlockCommissionBasisPoints: 1234,
		EpochCommissionBasisPoints: 5678,
		CommissionConfigured:       true,
		TotalWeight:                big.NewInt(3_500_000),
		FreezeHeight:               909_090,
		SelfStakeBucketIdx:         7,
	}
	buf, err := orig.Serialize()
	r.NoError(err)
	encoded, err := orig.Encode()
	r.NoError(err)
	r.Equal(buf, encoded.PrimaryData, "Erigon and trie storage must use identical bytes")

	var round CandidateRewardSnapshot
	r.NoError(round.Deserialize(buf))
	out := &round
	var genericRound CandidateRewardSnapshot
	r.NoError(genericRound.Decode(encoded))
	r.Zero(orig.TotalWeight.Cmp(genericRound.TotalWeight))

	r.Equal(orig.BlockCommissionBasisPoints, out.BlockCommissionBasisPoints)
	r.Equal(orig.EpochCommissionBasisPoints, out.EpochCommissionBasisPoints)
	r.Equal(orig.CommissionConfigured, out.CommissionConfigured)
	r.Zero(orig.TotalWeight.Cmp(out.TotalWeight))
	r.Equal(orig.FreezeHeight, out.FreezeHeight)
	r.Equal(orig.SelfStakeBucketIdx, out.SelfStakeBucketIdx)
}

func TestCandidateRewardSnapshot_SerializeZeroTotalWeight(t *testing.T) {
	// An opted-in candidate with frozen Votes=0 must round-trip to a zero,
	// non-nil TotalWeight. Rewarding checks Sign() to decide whether the
	// candidate has anything to distribute this era.
	r := require.New(t)
	orig := &CandidateRewardSnapshot{
		BlockCommissionBasisPoints: 1000,
		EpochCommissionBasisPoints: 2000,
		CommissionConfigured:       true,
	}
	buf, err := orig.Serialize()
	r.NoError(err)

	var round CandidateRewardSnapshot
	r.NoError(round.Deserialize(buf))
	out := &round
	r.NotNil(out.TotalWeight)
	r.Zero(out.TotalWeight.Sign())
	r.True(out.CommissionConfigured)
}

func TestCandidateRewardSnapshot_ZeroValueSerializes(t *testing.T) {
	// A zero-value CandidateRewardSnapshot must round-trip cleanly.
	r := require.New(t)
	orig := &CandidateRewardSnapshot{}
	buf, err := orig.Serialize()
	r.NoError(err)

	var pb stakingpb.CandidateRewardSnapshot
	r.NoError(proto.Unmarshal(buf, &pb))
	r.Zero(pb.GetBlockCommissionBasisPoints())
	r.Zero(pb.GetEpochCommissionBasisPoints())
	r.False(pb.GetCommissionConfigured())
	r.Empty(pb.GetTotalWeight())
	r.Zero(pb.GetFreezeHeight())
}

func TestCandidateRewardSnapshotKey_Layout(t *testing.T) {
	r := require.New(t)
	// Key is exactly 1 tag byte + candidate address bytes; tag byte is the
	// _candidateRewardSnapshot constant. This test guards against accidental
	// reuse of another byte for the same key layout.
	candID := identityset.Address(3)
	key := candidateRewardSnapshotKey(candID)
	r.Equal(1+len(candID.Bytes()), len(key))
	r.Equal(_candidateRewardSnapshot, key[0])
	r.Equal(candID.Bytes(), key[1:])
}

func TestCandidateRewardAddress(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	cand := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(2), Reward: identityset.Address(3),
		Name: "delegate", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	installCandCenter(t, sm, cand)
	csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
	r.NoError(err)
	r.NoError(csm.Upsert(cand))
	got, explicitlySet, err := CandidateRewardAddress(sm, cand.GetIdentifier())
	r.NoError(err)
	r.True(address.Equal(cand.Owner, got))
	r.False(explicitlySet)

	cand.Owner = identityset.Address(7)
	r.NoError(csm.Upsert(cand))
	got, explicitlySet, err = CandidateRewardAddress(sm, cand.GetIdentifier())
	r.NoError(err)
	r.True(address.Equal(cand.Owner, got), "default reward address must follow owner transfers")
	r.False(explicitlySet)

	cand.RewardAddressUpdated = true
	r.NoError(csm.Upsert(cand))
	got, explicitlySet, err = CandidateRewardAddress(sm, cand.GetIdentifier())
	r.NoError(err)
	r.True(address.Equal(cand.Reward, got))
	r.True(explicitlySet)

	_, _, err = CandidateRewardAddress(sm, identityset.Address(9))
	r.ErrorIs(err, state.ErrStateNotExist)
}

// A legacy candidate -- one that pays its voters off-chain -- is enumerated by
// the freezer along with everyone else and then dropped on the opt-in test,
// before either the profile read or the snapshot write. Both omissions are
// asserted: the reader fails the test if called, and the candidate ends the
// boundary with no record at all.
func TestFreezeCandidateRewardSnapshots_LegacyCandidateSkipsProfileAndSnapshot(t *testing.T) {
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
	r.NoError(FreezeCandidateRewardSnapshots(ctx, sm, bridge, reader, 0))

	_, err = CandidateRewardSnapshotFor(sm, candidate.GetIdentifier())
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist)
}

func TestFreezeCandidateRewardSnapshots_NilBridge(t *testing.T) {
	// Bridge nil path: the snapshot writer still runs (post-fork, no
	// contract configured), records CommissionConfigured=false and full owner
	// commission.
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

	r.NoError(FreezeCandidateRewardSnapshots(context.Background(), sm, nil, nil, 0))

	snap, err := CandidateRewardSnapshotFor(sm, firstCand.Owner)
	r.NoError(err)
	r.False(snap.CommissionConfigured)
	r.Equal(uint64(10000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(10000), snap.EpochCommissionBasisPoints)

	snap, err = CandidateRewardSnapshotFor(sm, secondCand.Owner)
	r.NoError(err)
	r.False(snap.CommissionConfigured)
}

func TestFreezeCandidateRewardSnapshots_HappyPath(t *testing.T) {
	// End-to-end path with a working bridge + reader: rates come back
	// CommissionConfigured=true with the expected inversion from voter-take portion
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

	r.NoError(FreezeCandidateRewardSnapshots(context.Background(), sm, bridge, fake.reader(t), 0))

	snap, err := CandidateRewardSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.True(snap.CommissionConfigured)
	r.Equal(uint64(1000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(2000), snap.EpochCommissionBasisPoints)
}

func TestFreezeCandidateRewardSnapshots_ExplicitZeroProfileIsConfigured(t *testing.T) {
	r := require.New(t)
	sm := testdb.NewMockStateManager(gomock.NewController(t))
	csm := newCandidateStateManager(sm)
	candidate := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(2),
		Reward: identityset.Address(3), Name: "zero-voter-portion",
		Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	r.NoError(putOnchainCandidate(csm, candidate))
	installCandCenter(t, sm, candidate)

	fake := newFakeProfileStore()
	fake.setPortion(candidate.Owner, "blockRewardPortion", 0)
	fake.setPortion(candidate.Owner, "epochRewardPortion", 0)
	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)
	r.NoError(FreezeCandidateRewardSnapshots(context.Background(), sm, bridge, fake.reader(t), 0))

	snapshot, err := CandidateRewardSnapshotFor(sm, candidate.Owner)
	r.NoError(err)
	r.True(snapshot.CommissionConfigured)
	r.Equal(_fullCommissionBasisPoints, snapshot.BlockCommissionBasisPoints)
	r.Equal(_fullCommissionBasisPoints, snapshot.EpochCommissionBasisPoints)
}

func TestFreezeCandidateRewardSnapshots_UsesCandidateIdentity(t *testing.T) {
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

	r.NoError(FreezeCandidateRewardSnapshots(context.Background(), sm, bridge, fake.reader(t), 0))

	snapshot, err := CandidateRewardSnapshotFor(sm, identity)
	r.NoError(err)
	r.True(snapshot.CommissionConfigured)
	_, err = CandidateRewardSnapshotFor(sm, operator)
	r.ErrorIs(err, state.ErrStateNotExist)
}

func TestFreezeCandidateRewardSnapshots_PartialProfile(t *testing.T) {
	// One delegate has a complete commission profile, another is absent from DelegateProfile.
	// The unconfigured one still gets a snapshot row (CommissionConfigured=false),
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

	r.NoError(FreezeCandidateRewardSnapshots(context.Background(), sm, bridge, fake.reader(t), 0))

	snap, err := CandidateRewardSnapshotFor(sm, registered.Owner)
	r.NoError(err)
	r.True(snap.CommissionConfigured)
	r.Equal(uint64(9500), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(9250), snap.EpochCommissionBasisPoints)

	snap, err = CandidateRewardSnapshotFor(sm, unregistered.Owner)
	r.NoError(err)
	r.False(snap.CommissionConfigured)
	r.Equal(uint64(10000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(10000), snap.EpochCommissionBasisPoints)
}

func TestFreezeCandidateRewardSnapshots_BridgeErrorDefaultsToFullOwnerCommission(t *testing.T) {
	// A per-delegate bridge failure must NOT abort the block. Any single
	// delegate's read error deterministically halts the chain at every epoch
	// boundary — worse than the reward-misroute it would prevent. The
	// snapshot is still written with CommissionConfigured=false and full owner commission.
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

	r.NoError(FreezeCandidateRewardSnapshots(context.Background(), sm, bridge, failing, 0))

	snap, err := CandidateRewardSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.False(snap.CommissionConfigured)
	r.Equal(uint64(10000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(10000), snap.EpochCommissionBasisPoints)
}

// The old TestFreezeCandidateRewardSnapshots_InvalidCandidateAddress lived here. It
// asserted that an unparseable identity in the poll list aborted the freeze;
// that parse no longer exists, because identities now come from the candidate
// center as address.Address values that were parsed when the candidate was
// registered.

func TestFreezeCandidateRewardSnapshots_NilBridgeSkipsReader(t *testing.T) {
	// A reader with no bridge is not an error -- there is nothing to read
	// against -- but it must not be invoked. The empty candidate center makes
	// this the empty-era case too: a boundary that finds no opted-in candidate
	// succeeds; the poll-layer caller opens the window as a separate next step.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	installCandCenter(t, sm)

	// Reader that would fail the test if called — proves nil-bridge path skips it.
	panicReader := delegateprofile.ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		t.Fatalf("reader must not be called when bridge is nil")
		return nil, nil
	})
	r.NoError(FreezeCandidateRewardSnapshots(context.Background(), sm, nil, panicReader, 0))
}

func TestFreezeCandidateRewardSnapshots_NonNilBridgeRequiresReader(t *testing.T) {
	// The wiring check runs before the freezer touches state, so this case
	// needs no view: a bridge with no reader is a caller bug, not a state
	// condition.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)
	err = FreezeCandidateRewardSnapshots(context.Background(), sm, bridge, nil, 0)
	r.Error(err)
	r.Contains(err.Error(), "nil ContractReader")
}

func TestCandidateRewardSnapshotFor_MissingReturnsErrStateNotExist(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	_, err := CandidateRewardSnapshotFor(sm, identityset.Address(1))
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist)
}

func TestCandidateRewardSnapshotFor_NilCandidateRejected(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	_, err := CandidateRewardSnapshotFor(sm, nil)
	r.Error(err)
}

// TestFreezeCandidateRewardSnapshots_EveryMemberGetsItsOwnRates is the multi-delegate
// bridge case: each member of the frozen set is looked up separately and each
// gets its own rates back, with no bleed between them. The order the bridge
// sees is pinned separately, by
// TestFreezeCandidateRewardSnapshots_FrozenSetOrderIsDeterministic.
func TestFreezeCandidateRewardSnapshots_EveryMemberGetsItsOwnRates(t *testing.T) {
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

	r.NoError(FreezeCandidateRewardSnapshots(context.Background(), sm, bridge, fake.reader(t), 0))
	for i, c := range cands {
		snap, err := CandidateRewardSnapshotFor(sm, c.Owner)
		r.NoError(err)
		r.True(snap.CommissionConfigured)
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
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(bp))
	s.values[storeKey(delegate, field)] = buf
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
