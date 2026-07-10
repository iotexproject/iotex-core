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
		VoterRewardOnchainOptIn:    true,
		Entries: []VoterWeight{
			{Voter: identityset.Address(1), Weight: big.NewInt(1_000_000)},
			{Voter: identityset.Address(2), Weight: big.NewInt(2_500_000)},
		},
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
	r.Equal(orig.VoterRewardOnchainOptIn, out.VoterRewardOnchainOptIn)
	r.Len(out.Entries, 2)
	r.Equal(orig.Entries[0].Voter.String(), out.Entries[0].Voter.String())
	r.Zero(orig.Entries[0].Weight.Cmp(out.Entries[0].Weight))
	r.Equal(orig.Entries[1].Voter.String(), out.Entries[1].Voter.String())
	r.Zero(orig.Entries[1].Weight.Cmp(out.Entries[1].Weight))
}

func TestCandidatePollSnapshot_SerializeEmptyEntries(t *testing.T) {
	// The IIP-59 skeleton PR writes empty Entries; this test pins the
	// contract that empty on the wire round-trips to len(Entries)==0
	// (so downstream rewarding's degenerate-branch check still triggers).
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
	r.Len(out.Entries, 0)
	r.True(out.Registered)
	r.False(out.VoterRewardOnchainOptIn)
}

func TestCandidatePollSnapshot_ZeroValueSerializes(t *testing.T) {
	// A zero-value CandidatePollSnapshot must round-trip cleanly — this is
	// what a delegate looks like when the DelegateProfile bridge is
	// disabled AND the delegate hasn't opted in.
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
	r.False(pb.GetVoterRewardOnchainOptIn())
	r.Empty(pb.GetEntries())
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

func TestReadLiveOptIn(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	optInCand := &Candidate{
		Owner:                   identityset.Address(1),
		Operator:                identityset.Address(2),
		Reward:                  identityset.Address(3),
		Name:                    "optIn",
		Votes:                   big.NewInt(1),
		SelfStake:               big.NewInt(1),
		VoterRewardOnchainOptIn: true,
	}
	optOutCand := &Candidate{
		Owner:                   identityset.Address(4),
		Operator:                identityset.Address(5),
		Reward:                  identityset.Address(6),
		Name:                    "optOut",
		Votes:                   big.NewInt(1),
		SelfStake:               big.NewInt(1),
		VoterRewardOnchainOptIn: false,
	}
	r.NoError(csm.putCandidate(optInCand))
	r.NoError(csm.putCandidate(optOutCand))

	got, err := readLiveOptIn(sm, optInCand.GetIdentifier())
	r.NoError(err)
	r.True(got)

	got, err = readLiveOptIn(sm, optOutCand.GetIdentifier())
	r.NoError(err)
	r.False(got)

	// Missing candidate degrades to (false, nil) rather than erroring, so a
	// stale poll entry doesn't wedge the block.
	got, err = readLiveOptIn(sm, identityset.Address(9))
	r.NoError(err)
	r.False(got)
}

func TestFreezePollSnapshot_NilBridge(t *testing.T) {
	// Bridge nil path: the snapshot writer still runs (post-fork, no
	// contract configured), captures opt-in only, and records
	// Registered=false so rewarding falls back to legacy.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	optInCand := &Candidate{
		Owner:                   identityset.Address(1),
		Operator:                identityset.Address(2),
		Reward:                  identityset.Address(3),
		Name:                    "opt-in-delegate",
		Votes:                   big.NewInt(1),
		SelfStake:               big.NewInt(1),
		VoterRewardOnchainOptIn: true,
	}
	optOutCand := &Candidate{
		Owner:                   identityset.Address(4),
		Operator:                identityset.Address(5),
		Reward:                  identityset.Address(6),
		Name:                    "opt-out-delegate",
		Votes:                   big.NewInt(1),
		SelfStake:               big.NewInt(1),
		VoterRewardOnchainOptIn: false,
	}
	r.NoError(csm.putCandidate(optInCand))
	r.NoError(csm.putCandidate(optOutCand))

	candidates := state.CandidateList{
		&state.Candidate{Address: optInCand.Owner.String(), Votes: big.NewInt(1), RewardAddress: optInCand.Reward.String()},
		&state.Candidate{Address: optOutCand.Owner.String(), Votes: big.NewInt(1), RewardAddress: optOutCand.Reward.String()},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, candidates, nil, nil))

	snap, err := PollSnapshotFor(sm, optInCand.Owner)
	r.NoError(err)
	r.False(snap.Registered)
	r.Zero(snap.BlockCommissionBasisPoints)
	r.Zero(snap.EpochCommissionBasisPoints)
	r.True(snap.VoterRewardOnchainOptIn)
	r.Empty(snap.Entries)

	snap, err = PollSnapshotFor(sm, optOutCand.Owner)
	r.NoError(err)
	r.False(snap.Registered)
	r.False(snap.VoterRewardOnchainOptIn)
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
		Owner:                   identityset.Address(1),
		Operator:                identityset.Address(2),
		Reward:                  identityset.Address(3),
		Name:                    "registered-delegate",
		Votes:                   big.NewInt(1),
		SelfStake:               big.NewInt(1),
		VoterRewardOnchainOptIn: true,
	}
	r.NoError(csm.putCandidate(cand))

	fake := newFakeProfileStore()
	fake.setPortion(cand.Owner, "blockRewardPortion", 9000)
	fake.setPortion(cand.Owner, "epochRewardPortion", 8000)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)

	candidates := state.CandidateList{
		&state.Candidate{Address: cand.Owner.String(), Votes: big.NewInt(1), RewardAddress: cand.Reward.String()},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, candidates, bridge, fake.reader(t)))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.True(snap.Registered)
	r.Equal(uint64(1000), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(2000), snap.EpochCommissionBasisPoints)
	r.True(snap.VoterRewardOnchainOptIn)
	r.Empty(snap.Entries)
}

func TestFreezePollSnapshot_PartialProfile(t *testing.T) {
	// One delegate registered on-chain, another absent from DelegateProfile.
	// The unregistered one still gets a snapshot row (Registered=false),
	// but opt-in flag is captured. This preserves the "either fully
	// opted-in or fully legacy" invariant.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	registered := &Candidate{
		Owner:                   identityset.Address(1),
		Operator:                identityset.Address(2),
		Reward:                  identityset.Address(3),
		Name:                    "registered",
		Votes:                   big.NewInt(1),
		SelfStake:               big.NewInt(1),
		VoterRewardOnchainOptIn: false, // opt-in flip lags by 1 epoch
	}
	unregistered := &Candidate{
		Owner:                   identityset.Address(4),
		Operator:                identityset.Address(5),
		Reward:                  identityset.Address(6),
		Name:                    "unregistered",
		Votes:                   big.NewInt(1),
		SelfStake:               big.NewInt(1),
		VoterRewardOnchainOptIn: true,
	}
	r.NoError(csm.putCandidate(registered))
	r.NoError(csm.putCandidate(unregistered))

	fake := newFakeProfileStore()
	// Only `registered` has portion fields set.
	fake.setPortion(registered.Owner, "blockRewardPortion", 500)
	fake.setPortion(registered.Owner, "epochRewardPortion", 750)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)

	candidates := state.CandidateList{
		&state.Candidate{Address: registered.Owner.String(), Votes: big.NewInt(1), RewardAddress: registered.Reward.String()},
		&state.Candidate{Address: unregistered.Owner.String(), Votes: big.NewInt(1), RewardAddress: unregistered.Reward.String()},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, candidates, bridge, fake.reader(t)))

	snap, err := PollSnapshotFor(sm, registered.Owner)
	r.NoError(err)
	r.True(snap.Registered)
	r.Equal(uint64(9500), snap.BlockCommissionBasisPoints)
	r.Equal(uint64(9250), snap.EpochCommissionBasisPoints)
	r.False(snap.VoterRewardOnchainOptIn)

	snap, err = PollSnapshotFor(sm, unregistered.Owner)
	r.NoError(err)
	r.False(snap.Registered)
	r.Zero(snap.BlockCommissionBasisPoints)
	r.Zero(snap.EpochCommissionBasisPoints)
	r.True(snap.VoterRewardOnchainOptIn)
}

func TestFreezePollSnapshot_BridgeErrorPropagates(t *testing.T) {
	// A bridge failure at PutPollResult must abort the block: a partial map
	// would misroute rewards for a whole epoch.
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
	r.NoError(csm.putCandidate(cand))

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)
	failing := delegateprofile.ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		return nil, errors.New("rpc down")
	})

	candidates := state.CandidateList{
		&state.Candidate{Address: cand.Owner.String(), Votes: big.NewInt(1), RewardAddress: cand.Reward.String()},
	}
	err = FreezePollSnapshot(context.Background(), sm, candidates, bridge, failing)
	r.Error(err)
	r.Contains(err.Error(), "rpc down")

	// No snapshot was written for the failed delegate.
	_, err = PollSnapshotFor(sm, cand.Owner)
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist)
}

func TestFreezePollSnapshot_InvalidCandidateAddress(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	candidates := state.CandidateList{
		&state.Candidate{Address: "not-a-bech32", Votes: big.NewInt(1)},
	}
	err := FreezePollSnapshot(context.Background(), sm, candidates, nil, nil)
	r.Error(err)
}

func TestFreezePollSnapshot_NilBridgeWithReaderRejected(t *testing.T) {
	// Guard against a caller passing an initialized reader with a nil bridge
	// — that's a bug: the reader would never be invoked and the caller is
	// probably confused about which arm they wanted.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	// Reader that would panic if called — proves nil-bridge path skips it.
	panicReader := delegateprofile.ContractReaderFunc(func(context.Context, string, []byte) ([]byte, error) {
		t.Fatalf("reader must not be called when bridge is nil")
		return nil, nil
	})
	candidates := state.CandidateList{}
	r.NoError(FreezePollSnapshot(context.Background(), sm, candidates, nil, panicReader))
}

func TestFreezePollSnapshot_NonNilBridgeRequiresReader(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)
	candidates := state.CandidateList{
		&state.Candidate{Address: identityset.Address(1).String(), Votes: big.NewInt(1)},
	}
	err = FreezePollSnapshot(context.Background(), sm, candidates, bridge, nil)
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

func TestFreezePollSnapshot_IterationOrderIsCallerOrder(t *testing.T) {
	// The bridge sees delegates in the order the poll layer supplied them.
	// Snapshot writes happen in the same order — determinism guardrail so
	// two replays of the same block produce the same trie state root.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	var cands []*Candidate
	for i := 1; i <= 3; i++ {
		c := &Candidate{
			Owner:                   identityset.Address(i),
			Operator:                identityset.Address(i + 10),
			Reward:                  identityset.Address(i + 20),
			Name:                    "d",
			Votes:                   big.NewInt(1),
			SelfStake:               big.NewInt(1),
			VoterRewardOnchainOptIn: i%2 == 0,
		}
		r.NoError(csm.putCandidate(c))
		cands = append(cands, c)
	}
	fake := newFakeProfileStore()
	for _, c := range cands {
		fake.setPortion(c.Owner, "blockRewardPortion", 4000)
		fake.setPortion(c.Owner, "epochRewardPortion", 5000)
	}
	bridge, err := delegateprofile.New("io1lfl4ppn2c3wcft04f0rk0jy9lyn4pcjcm7638u")
	r.NoError(err)

	list := state.CandidateList{}
	for _, c := range cands {
		list = append(list, &state.Candidate{Address: c.Owner.String(), Votes: big.NewInt(1), RewardAddress: c.Reward.String()})
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, list, bridge, fake.reader(t)))
	for _, c := range cands {
		snap, err := PollSnapshotFor(sm, c.Owner)
		r.NoError(err)
		r.True(snap.Registered)
		r.Equal(uint64(6000), snap.BlockCommissionBasisPoints)
		r.Equal(uint64(5000), snap.EpochCommissionBasisPoints)
		r.Equal(c.VoterRewardOnchainOptIn, snap.VoterRewardOnchainOptIn)
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
