// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/eracow"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// These integration tests cover the staking side of IIP-59 frozen state: the
// COW window lifecycle, bucket high-water marks, voter scanning, and frozen
// readers. The storage-neutral mechanism itself is covered in eracow tests.

const eraTestFreezeHeight = uint64(1_000)

func eraTestSM(t *testing.T) protocol.StateManager {
	return testdb.NewMockStateManager(gomock.NewController(t))
}

// forkGateCtx builds a context at `height` with the IIP-59 fork gate either
// open or shut. Everything in the era copy-on-write layer is inert before
// activation, so almost every test here needs both sides of that gate.
func forkGateCtx(height uint64, activated bool) context.Context {
	g := genesis.TestDefault()
	if activated {
		g.ZanzibarBlockHeight = height
	}
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	return protocol.WithFeatureCtx(ctx)
}

// eraTestCSM builds a candidate state manager with only the pieces the bucket
// writers touch. The candidate center is not involved in any write this file
// exercises, and constructing one would drag in a view the test does not need.
func eraTestCSM(ctx context.Context, sm protocol.StateManager) *candSM {
	return &candSM{StateManager: sm, cow: eracow.NewSession(ctx, sm)}
}

func eraTestBucket(t *testing.T, owner, cand int, amount int64) *VoteBucket {
	t.Helper()
	return NewVoteBucket(
		identityset.Address(cand), identityset.Address(owner),
		big.NewInt(amount), 91, time.Now(), true,
	)
}

func eraTestContractBucket(owner, cand int, amount int64) *contractstaking.Bucket {
	return &contractstaking.Bucket{
		Candidate:        identityset.Address(cand),
		Owner:            identityset.Address(owner),
		StakedAmount:     big.NewInt(amount),
		StakedDuration:   86400,
		CreatedAt:        1,
		IsTimestampBased: true,
	}
}

func eraTestAddress(t *testing.T, prefix, suffix byte) address.Address {
	t.Helper()
	raw := make([]byte, 20)
	raw[0], raw[len(raw)-1] = prefix, suffix
	addr, err := address.FromBytes(raw)
	require.NoError(t, err)
	return addr
}

// TestBeginEraCOWWindowFreezesBucketIndexUpperBounds pins the two exclusive
// boundaries the drain uses to reject buckets that did not exist at the freeze
// height. The native value is already the next index to be handed out; the
// contract value converts its highest minted id to the same representation.
func TestBeginEraCOWWindowFreezesBucketIndexUpperBounds(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)

	// Three native buckets => next index is 3.
	for i := 0; i < 3; i++ {
		_, err := csm.putBucket(eraTestBucket(t, 2, 1, 100))
		r.NoError(err)
	}
	contract := identityset.Address(20)
	other := identityset.Address(21)
	cs := contractstaking.NewContractStakingStateManager(sm)
	r.NoError(cs.UpdateNumOfBuckets(contract, 7))

	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))

	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)
	r.True(window.Open())
	r.Equal(eraTestFreezeHeight, window.FreezeHeight)

	// Native: index 2 was assigned, index 3 is the next one out.
	r.True(window.NativeBucketExisted(2))
	r.False(window.NativeBucketExisted(3))

	// Contract: id 7 was minted, id 8 has not been.
	r.True(window.ContractBucketExisted(contract.Bytes(), 7))
	r.False(window.ContractBucketExisted(contract.Bytes(), 8))
	// Id 0 is a legal contract bucket id in the state layer, so the check must
	// not treat it as an absence.
	r.True(window.ContractBucketExisted(contract.Bytes(), 0))
	// A contract with no record at all admits nothing.
	r.False(window.ContractBucketExisted(other.Bytes(), 1))
}

// TestEraCOWWindowIsInertPreActivation is the consensus-safety test: before the
// fork gate opens the layer must not read or write a single key, because a
// stray write is a state root divergence between upgraded and un-upgraded
// nodes days before activation.
func TestEraCOWWindowIsInertPreActivation(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, false)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)

	_, err := csm.putBucketAndIndex(eraTestBucket(t, 2, 1, 100))
	r.NoError(err)
	before := eraTestStateCount(t, sm)

	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)
	r.False(window.Open())

	// Every covered mutation, with the gate shut.
	r.NoError(csm.updateBucket(0, eraTestBucket(t, 2, 1, 200)))
	r.NoError(csm.delBucketAndIndex(identityset.Address(2), identityset.Address(1), 0))
	n, err := CollectEraCOWGarbage(ctx, sm, 100)
	r.NoError(err)
	r.Zero(n)
	r.NoError(SealEraCOWWindow(ctx, sm))

	// The deletes above remove keys; what must not happen is a key appearing
	// that the pre-fork binary would not have written.
	r.LessOrEqual(eraTestStateCount(t, sm), before)
	r.Zero(eraTestCOWKeyCount(t, sm))
}

// TestFrozenNativeBucketSeesValueAtFreezeHeight covers the case the whole layer
// exists for: the drain mutates the very buckets it is paying out (compound
// deposits grow StakedAmount) and must keep reading the pre-boundary value.
func TestFrozenNativeBucketSeesValueAtFreezeHeight(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)

	index, err := csm.putBucketAndIndex(eraTestBucket(t, 2, 1, 100))
	r.NoError(err)
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	grown := eraTestBucket(t, 2, 1, 100)
	grown.Index = index
	grown.StakedAmount = big.NewInt(999)
	r.NoError(csm.updateBucket(index, grown))

	live, err := csm.NativeBucket(index)
	r.NoError(err)
	r.Equal(big.NewInt(999), live.StakedAmount)

	frozen, err := newCandidateStateReader(sm).FrozenNativeBucket(window, index)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)
	r.Equal(index, frozen.Index)

	// A second write must not overwrite the first copy — the era's value is the
	// one from before the *first* touch.
	grown.StakedAmount = big.NewInt(1_500)
	r.NoError(csm.updateBucket(index, grown))
	frozen, err = newCandidateStateReader(sm).FrozenNativeBucket(window, index)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)
}

// TestFrozenNativeBucketSurvivesWithdrawal covers the other volatile direction:
// the bucket and the voter's index entry are both deleted, and the drain still
// has to pay the voter who owned them at the boundary.
func TestFrozenNativeBucketSurvivesWithdrawal(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	voter, cand := identityset.Address(2), identityset.Address(1)

	index, err := csm.putBucketAndIndex(eraTestBucket(t, 2, 1, 100))
	r.NoError(err)
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	r.NoError(csm.delBucketAndIndex(voter, cand, index))

	_, err = csm.NativeBucket(index)
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist)

	frozen, err := newCandidateStateReader(sm).FrozenNativeBucket(window, index)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)

	indices, err := newCandidateStateReader(sm).FrozenNativeBucketIndices(window, voter)
	r.NoError(err)
	r.Equal(BucketIndices{index}, indices)
}

func TestScanFrozenVotersCountsVotersNotBuckets(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	cand := identityset.Address(1)
	voters := []address.Address{
		eraTestAddress(t, 0x10, 1),
		eraTestAddress(t, 0x40, 2),
		eraTestAddress(t, 0x70, 3),
	}

	var withdrawn uint64
	for i, voter := range voters {
		buckets := 1
		if i == 1 {
			buckets = 3
		}
		for j := 0; j < buckets; j++ {
			bucket := NewVoteBucket(cand, voter, big.NewInt(int64(100+j)), 91, time.Now(), true)
			index, err := csm.putBucketAndIndex(bucket)
			r.NoError(err)
			if i == 2 {
				withdrawn = index
			}
		}
	}
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	// Delete the third voter's only live index entry. Its frozen voter index is
	// now available only through COW and must still be enumerated.
	r.NoError(csm.delBucketAndIndex(voters[2], cand, withdrawn))

	first, err := ScanFrozenVoters(sm, window, make([]byte, 20), nil, nil, 2, 100)
	r.NoError(err)
	r.False(first.Complete)
	r.Equal(voters[:2], first.Voters,
		"three buckets owned by one voter must consume one voter slot")

	second, err := ScanFrozenVoters(sm, window, make([]byte, 20), nil, first.ResumeAfter, 2, 100)
	r.NoError(err)
	r.True(second.Complete)
	r.Equal([]address.Address{voters[2]}, second.Voters,
		"a voter deleted from the live index must be recovered from COW")
}

func TestScanFrozenVotersAdvancesAcrossCOWTombstones(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	cand := identityset.Address(1)
	owed := eraTestAddress(t, 0xf0, 1)
	owedBucket := NewVoteBucket(cand, owed, big.NewInt(100), 91, time.Now(), true)
	_, err := csm.putBucketAndIndex(owedBucket)
	r.NoError(err)
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	// These voters enter and fully exit after the freeze. Their live index keys
	// are gone and their COW entries are tombstones, so they consume scan work
	// without becoming voters in the frozen page.
	for i := 0; i < 10; i++ {
		voter := eraTestAddress(t, 0x10, byte(i+1))
		bucket := NewVoteBucket(cand, voter, big.NewInt(100), 91, time.Now(), true)
		index, err := csm.putBucketAndIndex(bucket)
		r.NoError(err)
		r.NoError(csm.delBucketAndIndex(voter, cand, index))
	}

	var (
		resume     []byte
		got        []address.Address
		emptyPages int
	)
	for i := 0; i < 20; i++ {
		page, err := ScanFrozenVoters(sm, window, make([]byte, 20), nil, resume, 2, 8)
		r.NoError(err)
		if len(page.Voters) == 0 && !page.Complete {
			emptyPages++
		}
		got = append(got, page.Voters...)
		resume = page.ResumeAfter
		if page.Complete {
			break
		}
	}
	r.Positive(emptyPages, "tombstone-only pages must advance without inventing voters")
	r.Equal([]address.Address{owed}, got)
}

func TestScanFrozenVotersDoesNotCountPostFreezeVoters(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	cand := identityset.Address(1)
	owed := eraTestAddress(t, 0xf0, 1)
	_, err := csm.putBucketAndIndex(NewVoteBucket(cand, owed, big.NewInt(100), 91, time.Now(), true))
	r.NoError(err)
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	// The newcomer has a live index, but its COW tombstone says that index did
	// not exist at the freeze height. It must not consume the single voter slot.
	newcomer := eraTestAddress(t, 0x10, 1)
	_, err = csm.putBucketAndIndex(NewVoteBucket(cand, newcomer, big.NewInt(100), 91, time.Now(), true))
	r.NoError(err)
	page, err := ScanFrozenVoters(sm, window, make([]byte, 20), nil, nil, 1, 100)
	r.NoError(err)
	r.Equal([]address.Address{owed}, page.Voters)
	next, err := ScanFrozenVoters(sm, window, make([]byte, 20), nil, page.ResumeAfter, 1, 100)
	r.NoError(err)
	r.True(next.Complete)
	r.Empty(next.Voters)
}

func TestScanFrozenVotersAppliesCOWPerIndexFamily(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	cs := contractstaking.NewContractStakingStateManager(sm)
	cand := identityset.Address(1)
	contract := identityset.Address(20)
	nativeAtFreeze := eraTestAddress(t, 0x40, 1)
	contractAtFreeze := eraTestAddress(t, 0x80, 1)

	_, err := csm.putBucketAndIndex(NewVoteBucket(cand, nativeAtFreeze, big.NewInt(100), 91, time.Now(), true))
	r.NoError(err)
	r.NoError(cs.UpsertBucket(ctx, contract, 1, &contractstaking.Bucket{
		Owner: contractAtFreeze, Candidate: cand, StakedAmount: big.NewInt(100),
	}))
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	// Each voter enters the other index family only after the freeze. Those
	// family-specific tombstones must not hide the index that did exist at H.
	r.NoError(cs.UpsertBucket(ctx, contract, 2, &contractstaking.Bucket{
		Owner: nativeAtFreeze, Candidate: cand, StakedAmount: big.NewInt(100),
	}))
	_, err = csm.putBucketAndIndex(NewVoteBucket(cand, contractAtFreeze, big.NewInt(100), 91, time.Now(), true))
	r.NoError(err)

	page, err := ScanFrozenVoters(sm, window, make([]byte, 20), nil, nil, 10, 100)
	r.NoError(err)
	r.True(page.Complete)
	r.Equal([]address.Address{nativeAtFreeze, contractAtFreeze}, page.Voters)
}

func TestScanFrozenVotersRangeAndResumeAreExclusive(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	cand := identityset.Address(1)
	voters := []address.Address{
		eraTestAddress(t, 0x10, 1),
		eraTestAddress(t, 0x40, 2),
		eraTestAddress(t, 0x70, 3),
		eraTestAddress(t, 0x90, 4),
	}
	for _, voter := range voters {
		_, err := csm.putBucketAndIndex(NewVoteBucket(
			cand, voter, big.NewInt(100), 91, time.Now(), true,
		))
		r.NoError(err)
	}
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	page, err := ScanFrozenVoters(
		sm, window, voters[1].Bytes(), voters[3].Bytes(), voters[1].Bytes(), 0, 0,
	)
	r.NoError(err)
	r.True(page.Complete)
	r.Equal([]address.Address{voters[2]}, page.Voters,
		"range start is inclusive, range end and resume cursor are exclusive")
	r.Equal(voters[2].Bytes(), page.ResumeAfter)
}

func TestScanFrozenVotersDeduplicatesIndexFamilies(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	cs := contractstaking.NewContractStakingStateManager(sm)
	voter := eraTestAddress(t, 0x40, 1)
	cand := identityset.Address(1)
	contract := identityset.Address(20)

	_, err := csm.putBucketAndIndex(NewVoteBucket(
		cand, voter, big.NewInt(100), 91, time.Now(), true,
	))
	r.NoError(err)
	r.NoError(cs.UpsertBucket(ctx, contract, 1, &contractstaking.Bucket{
		Owner: voter, Candidate: cand, StakedAmount: big.NewInt(100),
	}))
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	page, err := ScanFrozenVoters(sm, window, make([]byte, _addressLength), nil, nil, 0, 0)
	r.NoError(err)
	r.True(page.Complete)
	r.Equal([]address.Address{voter}, page.Voters,
		"one voter present in native and contract indexes must be settled once")
}

func TestScanFrozenVotersUsesFrozenContractOwner(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	cs := contractstaking.NewContractStakingStateManager(sm)
	contract := identityset.Address(20)
	cand := identityset.Address(1)
	oldOwner := eraTestAddress(t, 0x20, 1)
	newOwner := eraTestAddress(t, 0x80, 1)

	r.NoError(cs.UpsertBucket(ctx, contract, 1, &contractstaking.Bucket{
		Owner: oldOwner, Candidate: cand, StakedAmount: big.NewInt(100),
	}))
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)
	r.NoError(cs.UpsertBucket(ctx, contract, 1, &contractstaking.Bucket{
		Owner: newOwner, Candidate: cand, StakedAmount: big.NewInt(100),
	}))

	page, err := ScanFrozenVoters(sm, window, make([]byte, _addressLength), nil, nil, 0, 0)
	r.NoError(err)
	r.True(page.Complete)
	r.Equal([]address.Address{oldOwner}, page.Voters,
		"an owner transfer after the freeze must not move the era reward")
}

func TestScanFrozenVotersRejectsInvalidRange(t *testing.T) {
	window := eracow.Window{FreezeHeight: eraTestFreezeHeight}
	start := make([]byte, _addressLength)
	end := bytes.Repeat([]byte{0x80}, _addressLength)

	tests := []struct {
		name   string
		start  []byte
		end    []byte
		resume []byte
	}{
		{name: "short start", start: start[:_addressLength-1]},
		{name: "short end", start: start, end: end[:_addressLength-1]},
		{name: "reversed range", start: end, end: start},
		{name: "short resume", start: start, end: end, resume: start[:_addressLength-1]},
		{name: "resume before start", start: end, resume: start},
		{name: "resume at end", start: start, end: end, resume: end},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ScanFrozenVoters(eraTestSM(t), window, tt.start, tt.end, tt.resume, 1, 4)
			require.Error(t, err)
		})
	}
}

func TestFrozenCandidatesForVoterUsesFrozenBucketsAndDeduplicates(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	cs := contractstaking.NewContractStakingStateManager(sm)
	voter := identityset.Address(2)
	contract := identityset.Address(20)
	lowCandidate := eraTestAddress(t, 0x10, 1)
	highCandidate := eraTestAddress(t, 0x90, 1)

	nativeIndex, err := csm.putBucketAndIndex(NewVoteBucket(
		highCandidate, voter, big.NewInt(100), 91, time.Now(), true,
	))
	r.NoError(err)
	r.NoError(cs.UpsertBucket(ctx, contract, 1, &contractstaking.Bucket{
		Owner: voter, Candidate: lowCandidate, StakedAmount: big.NewInt(100),
	}))
	r.NoError(cs.UpsertBucket(ctx, contract, 2, &contractstaking.Bucket{
		Owner: voter, Candidate: highCandidate, StakedAmount: big.NewInt(100),
	}))
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	updated := NewVoteBucket(lowCandidate, voter, big.NewInt(100), 91, time.Now(), true)
	updated.Index = nativeIndex
	r.NoError(csm.updateBucket(nativeIndex, updated))
	r.NoError(cs.DeleteBucket(ctx, contract, 1))

	candidates, err := FrozenCandidatesForVoter(sm, window, voter)
	r.NoError(err)
	r.Equal([]address.Address{lowCandidate, highCandidate}, candidates,
		"candidate discovery must use frozen values, sort them, and remove duplicates")
}

// TestFrozenNativeBucketRejectsPostFreezeIndex pins the high-water mark: a
// bucket created after the boundary is not payable in that era, and the voter
// index entry it created reads as empty rather than as the post-boundary list.
func TestFrozenNativeBucketRejectsPostFreezeIndex(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)

	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	newcomer := identityset.Address(5)
	index, err := csm.putBucketAndIndex(eraTestBucket(t, 5, 1, 100))
	r.NoError(err)
	r.Zero(index, "the window was opened before any bucket existed")

	_, err = newCandidateStateReader(sm).FrozenNativeBucket(window, index)
	r.ErrorIs(err, eracow.ErrBucketPostFreeze)
	_, err = sm.State(&eracow.Entry{},
		protocol.NamespaceOption(eracow.Namespace),
		protocol.KeyOption(eracow.EntryKey(window.FreezeHeight, eracow.KindNativeBucket, eracow.NativeBucketSubkey(index))),
	)
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist, "the native bound makes a bucket tombstone redundant")

	indices, err := newCandidateStateReader(sm).FrozenNativeBucketIndices(window, newcomer)
	r.NoError(err)
	r.Empty(indices, "a voter who first staked after the boundary has no frozen list")
}

// TestFrozenNativeBucketRejectsPostFreezeCreateThenUpdate covers the native
// create path followed by ordinary bucket updates in the same COW window. The
// frozen upper bound remains authoritative: updating a post-freeze bucket must
// neither make it visible to the frozen era nor create a redundant COW entry.
// The voter's pre-freeze index list must also remain unchanged.
func TestFrozenNativeBucketRejectsPostFreezeCreateThenUpdate(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	voter, cand := identityset.Address(2), identityset.Address(1)

	preFreeze, err := csm.putBucketAndIndex(eraTestBucket(t, 2, 1, 100))
	r.NoError(err)
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	postFreeze, err := csm.putBucketAndIndex(eraTestBucket(t, 2, 1, 200))
	r.NoError(err)
	r.False(window.NativeBucketExisted(postFreeze))

	for _, amount := range []int64{300, 400} {
		updated := eraTestBucket(t, 2, 1, amount)
		updated.Index = postFreeze
		r.NoError(csm.updateBucket(postFreeze, updated))
	}

	live, err := csm.NativeBucket(postFreeze)
	r.NoError(err)
	r.Equal(big.NewInt(400), live.StakedAmount)
	_, err = newCandidateStateReader(sm).FrozenNativeBucket(window, postFreeze)
	r.ErrorIs(err, eracow.ErrBucketPostFreeze)

	_, err = sm.State(&eracow.Entry{},
		protocol.NamespaceOption(eracow.Namespace),
		protocol.KeyOption(eracow.EntryKey(
			window.FreezeHeight, eracow.KindNativeBucket, eracow.NativeBucketSubkey(postFreeze),
		)),
	)
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist,
		"updates must not snapshot a bucket outside the frozen upper bound")

	indices, err := newCandidateStateReader(sm).FrozenNativeBucketIndices(window, voter)
	r.NoError(err)
	r.Equal(BucketIndices{preFreeze}, indices,
		"the post-freeze bucket must not enter the voter's frozen index list")

	frozen, err := newCandidateStateReader(sm).FrozenNativeBucket(window, preFreeze)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)
	r.Equal(cand.String(), frozen.Candidate.String())
}

// TestFrozenNativeBucketSurvivesPostFreezeOwnerTransfer verifies the three-key
// COW operation behind a native stake transfer: the bucket keeps its old owner,
// the old owner's index keeps the bucket, and the new owner's index remains
// absent in the frozen view.
func TestFrozenNativeBucketSurvivesPostFreezeOwnerTransfer(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)
	oldOwner, newOwner := identityset.Address(2), identityset.Address(3)
	cand := identityset.Address(1)

	index, err := csm.putBucketAndIndex(NewVoteBucket(
		cand, oldOwner, big.NewInt(100), 91, time.Now(), true,
	))
	r.NoError(err)
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	transferred := NewVoteBucket(cand, newOwner, big.NewInt(100), 91, time.Now(), true)
	transferred.Index = index
	r.NoError(csm.updateBucket(index, transferred))
	r.NoError(csm.delVoterBucketIndex(oldOwner, index))
	r.NoError(csm.putVoterBucketIndex(newOwner, index))

	live, err := csm.NativeBucket(index)
	r.NoError(err)
	r.Equal(newOwner.String(), live.Owner.String())

	frozen, err := newCandidateStateReader(sm).FrozenNativeBucket(window, index)
	r.NoError(err)
	r.Equal(oldOwner.String(), frozen.Owner.String())
	oldIndices, err := newCandidateStateReader(sm).FrozenNativeBucketIndices(window, oldOwner)
	r.NoError(err)
	r.Equal(BucketIndices{index}, oldIndices)
	newIndices, err := newCandidateStateReader(sm).FrozenNativeBucketIndices(window, newOwner)
	r.NoError(err)
	r.Empty(newIndices)
}

// TestFrozenContractBucketSeesValueAtFreezeHeight is the LSD counterpart:
// receipt processing rewrites these buckets on nearly every block.
func TestFrozenContractBucketSeesValueAtFreezeHeight(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	cs := contractstaking.NewContractStakingStateManager(sm)
	contract := identityset.Address(20)
	owner := identityset.Address(2)

	r.NoError(cs.UpsertBucket(ctx, contract, 3, eraTestContractBucket(2, 1, 100)))
	r.NoError(cs.UpdateNumOfBuckets(contract, 3))
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	r.NoError(cs.UpsertBucket(ctx, contract, 3, eraTestContractBucket(2, 1, 999)))
	frozen, err := contractstaking.NewStateReader(sm).FrozenBucket(window, contract, 3)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)

	// Burning the bucket must not take the era's copy with it.
	r.NoError(cs.DeleteBucket(ctx, contract, 3))
	frozen, err = contractstaking.NewStateReader(sm).FrozenBucket(window, contract, 3)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)

	// The owner index is emptied by the delete; the frozen list still names it.
	refs, err := contractstaking.NewStateReader(sm).FrozenBucketRefs(window, owner)
	r.NoError(err)
	r.Len(refs, 1)
	r.Equal(uint64(3), refs[0].BucketID)
}

// TestFrozenContractBucketRejectsPostFreezeID pins the exclusive upper bound.
// Contract bucket ids come from a strictly monotonic counter that burning does
// not touch, so "id >= the frozen bound" is exactly "minted after the boundary".
//
// The converse does not hold, and the mark is therefore only half the defence:
// see TestFrozenContractBucketRejectsPostFreezeIDInAGap.
func TestFrozenContractBucketRejectsPostFreezeID(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	cs := contractstaking.NewContractStakingStateManager(sm)
	contract := identityset.Address(20)

	r.NoError(cs.UpsertBucket(ctx, contract, 3, eraTestContractBucket(2, 1, 100)))
	r.NoError(cs.UpdateNumOfBuckets(contract, 3))
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	// Minted after the boundary.
	r.NoError(cs.UpsertBucket(ctx, contract, 4, eraTestContractBucket(6, 1, 100)))
	r.NoError(cs.UpdateNumOfBuckets(contract, 4))

	_, err = contractstaking.NewStateReader(sm).FrozenBucket(window, contract, 4)
	r.ErrorIs(err, eracow.ErrBucketPostFreeze)
	_, err = sm.State(&eracow.Entry{},
		protocol.NamespaceOption(eracow.Namespace),
		protocol.KeyOption(eracow.EntryKey(window.FreezeHeight, eracow.KindLSDBucket, eracow.LSDBucketSubkey(contract.Bytes(), 4))),
	)
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist, "the contract bound makes an above-bound tombstone redundant")
	// The last ID below the exclusive boundary remains valid.
	_, err = contractstaking.NewStateReader(sm).FrozenBucket(window, contract, 3)
	r.NoError(err)
}

// TestFrozenContractBucketRejectsPostFreezeIDInAGap covers the case the
// high-water mark cannot reach, and therefore the only case where the
// Exists=false tombstone is the sole thing standing between the drain and a
// bucket that did not exist at the freeze height.
//
// The mark is the highest id ever minted, not a count, and the id space below
// it is full of holes: mainnet burns buckets. An id minted into one of those
// holes after the boundary is <= the mark, so ContractBucketExisted admits it
// and Resolve is reached. Resolve falls through to the live value when there is
// no entry, so what stops it there is the tombstone that Snapshot writes when
// it finds prior == nil -- which is why UpsertBucket snapshots on the create
// path too, unlike its native counterpart putBucket, where indices come from a
// monotone counter and the mark alone is sufficient.
//
// Deleting the tombstone write in eracow's Snapshot turns three tests red, and
// this is the only one of them that is about a bucket *value*:
// eracow.TestFrozenReadResolution covers Resolve directly, one layer down, and
// TestFrozenNativeBucketRejectsPostFreezeIndex fails on its voter-index
// assertion, not its bucket one -- the native bucket there is caught by the
// mark either way. Nothing else in the staking package notices.
func TestFrozenContractBucketRejectsPostFreezeIDInAGap(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	cs := contractstaking.NewContractStakingStateManager(sm)
	contract := identityset.Address(20)
	newcomer := identityset.Address(6)

	// At the freeze height id 10 is live and id 5 is a hole -- minted and burnt
	// at some earlier point, which leaves the mark where it is.
	r.NoError(cs.UpsertBucket(ctx, contract, 10, eraTestContractBucket(2, 1, 100)))
	r.NoError(cs.UpdateNumOfBuckets(contract, 10))
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	// The precondition that makes this test worth having: the mark lets the gap
	// id through, so the rejection below cannot be coming from the mark.
	r.True(window.ContractBucketExisted(contract.Bytes(), 5),
		"the high-water mark admits an id below it, hole or not")

	// Minted into the gap after the boundary, by an owner who held nothing at
	// the freeze height.
	r.NoError(cs.UpsertBucket(ctx, contract, 5, eraTestContractBucket(6, 1, 999)))

	_, err = contractstaking.NewStateReader(sm).FrozenBucket(window, contract, 5)
	r.ErrorIs(err, eracow.ErrBucketPostFreeze,
		"a gap id minted after the boundary must not resolve to the live bucket")

	refs, err := contractstaking.NewStateReader(sm).FrozenBucketRefs(window, newcomer)
	r.NoError(err)
	r.Empty(refs, "an owner whose first LSD bucket is post-boundary has no frozen list")

	// The bucket that really was there is unaffected: the tombstone is a
	// per-key statement, not a per-contract one.
	frozen, err := contractstaking.NewStateReader(sm).FrozenBucket(window, contract, 10)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)
}

// TestSealEraCOWWindowStopsCopying pins the "no outstanding drain, no work"
// half of the design: once the drain completes, bucket writes stop paying for
// the copy hooks and the frozen readers stop answering.
func TestSealEraCOWWindowStopsCopying(t *testing.T) {
	r := require.New(t)
	ctx := forkGateCtx(eraTestFreezeHeight, true)
	sm := eraTestSM(t)
	csm := eraTestCSM(ctx, sm)

	index, err := csm.putBucketAndIndex(eraTestBucket(t, 2, 1, 100))
	r.NoError(err)
	r.NoError(BeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	grown := eraTestBucket(t, 2, 1, 100)
	grown.StakedAmount = big.NewInt(999)
	r.NoError(csm.updateBucket(index, grown))
	r.Positive(eraTestCOWKeyCount(t, sm))

	r.NoError(SealEraCOWWindow(ctx, sm))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)
	r.False(window.Open())

	// A fresh session (a new block would build one) writes nothing more.
	sealed := eraTestCOWKeyCount(t, sm)
	after := eraTestCSM(ctx, sm)
	grown.StakedAmount = big.NewInt(1_500)
	r.NoError(after.updateBucket(index, grown))
	r.Equal(sealed, eraTestCOWKeyCount(t, sm))

	// The sealed era's copies are collectable, and collection is bounded.
	n, err := CollectEraCOWGarbage(ctx, sm, 1)
	r.NoError(err)
	r.Equal(1, n)
	for {
		n, err = CollectEraCOWGarbage(ctx, sm, 8)
		r.NoError(err)
		if n == 0 {
			break
		}
	}
	r.Zero(eraTestCOWKeyCount(t, sm))
}

// eraTestStateCount counts every key in the staking namespace.
func eraTestStateCount(t *testing.T, sr protocol.StateReader) int {
	t.Helper()
	_, iter, err := sr.States(protocol.NamespaceOption(_stakingNameSpace))
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return 0
		}
		require.NoError(t, err)
	}
	return iter.Size()
}

// eraTestCOWKeyCount counts the keys the copy-on-write layer owns, by tag.
func eraTestCOWKeyCount(t *testing.T, sr protocol.StateReader) int {
	t.Helper()
	_, iter, err := sr.States(protocol.NamespaceOption(_stakingNameSpace))
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return 0
		}
		require.NoError(t, err)
	}
	count := 0
	for i := 0; i < iter.Size(); i++ {
		var raw eraTestRawValue
		key, err := iter.Next(&raw)
		if err != nil && !errors.Is(err, state.ErrNilValue) {
			require.NoError(t, err)
		}
		if len(key) == 0 {
			continue
		}
		switch key[0] {
		case eracow.ControlPrefix, eracow.EntryPrefix:
			count++
		}
	}
	return count
}

// TestFrozenContractBucketUnknownContract pins the Defect C behaviour at the
// read side: a contract with no frozen mark is denied, not allowed. Allowing
// would admit buckets minted after the freeze into a frozen era, which is worse
// than an under-payment; the deny is made noisy in the log instead.
func TestFrozenContractBucketUnknownContract(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	known, unknown := identityset.Address(20), identityset.Address(21)

	ctx := forkGateCtx(eraTestFreezeHeight, true)
	r.NoError(contractstaking.NewContractStakingStateManager(sm).UpdateNumOfBuckets(known, 4))
	r.NoError(TestOnlyBeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	r.True(window.ContractKnown(known.Bytes()))
	r.False(window.ContractKnown(unknown.Bytes()))
	r.True(window.ContractBucketExisted(known.Bytes(), 4))
	r.False(window.ContractBucketExisted(known.Bytes(), 5))
	r.False(window.ContractBucketExisted(unknown.Bytes(), 1))

	_, err = contractstaking.NewStateReader(sm).FrozenBucket(window, unknown, 1)
	r.ErrorIs(err, eracow.ErrBucketPostFreeze)
}

// TestSupersededWindowSilentlyAnswersForTheWrongEra pins the hazard that
// rewarding's superseded-window guard exists to prevent, at the layer where the
// hazard actually lives.
//
// eracow.Begin does not refuse a second era boundary arriving while the previous
// era's drain is still outstanding. It queues the old window for collection and
// installs the new one, so LoadEraCOWWindow starts answering at the new freeze
// height. Nothing here defends against being handed that window together with
// the old era's work: these readers take a window as an argument and answer for
// whatever height it names. That is the right contract for a primitive -- the
// window is the caller's statement of which era it is settling -- but it means
// the check has to be somewhere, and it cannot be here.
//
// What the drain would get, if it did not check, is not a stall and not an
// error. It is a settlement for era 1 computed on era 2's state, wrong in both
// directions the window can be wrong:
//
//   - a bucket that grew after H1 reads at its grown value, and
//   - a bucket that did not exist at H1 at all becomes readable.
//
// Era 1's copies are still on disk throughout, so this is a matter of reading
// the wrong tag rather than of losing data. The guard that acts on it lives in
// runVoterDistributionChunk; see TestVoterDrainRefusesASupersededEraWindow.
func TestSupersededWindowSilentlyAnswersForTheWrongEra(t *testing.T) {
	r := require.New(t)
	const (
		h1 = eraTestFreezeHeight      // era 1 freezes here
		h2 = eraTestFreezeHeight + 50 // era 2 freezes here, drain still running
	)
	// Activation stays at h1 for both contexts; only the block height advances.
	ctxAtH1 := forkGateCtx(h1, true)
	ctxAtH2 := protocol.WithFeatureCtx(protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), func() genesis.Genesis {
			g := genesis.TestDefault()
			g.ZanzibarBlockHeight = h1
			return g
		}()),
		protocol.BlockCtx{BlockHeight: h2},
	))
	sm := eraTestSM(t)
	csm := eraTestCSM(ctxAtH1, sm)

	// --- era 1 --------------------------------------------------------------
	staked, err := csm.putBucketAndIndex(eraTestBucket(t, 2, 1, 100))
	r.NoError(err)
	r.NoError(BeginEraCOWWindow(ctxAtH1, sm, h1))
	windowH1, err := LoadEraCOWWindow(sm)
	r.NoError(err)

	// The drain for era 1 starts. Mid-drain the bucket grows (a compound
	// deposit does exactly this) and a brand new bucket is staked.
	grown := eraTestBucket(t, 2, 1, 100)
	grown.Index = staked
	grown.StakedAmount = big.NewInt(999)
	r.NoError(csm.updateBucket(staked, grown))
	postFreeze, err := csm.putBucketAndIndex(eraTestBucket(t, 3, 1, 700))
	r.NoError(err)
	r.NotEqual(staked, postFreeze)

	// While era 1's window is the live one the drain is correct: it sees the
	// pre-mutation value and refuses the newcomer.
	frozen, err := newCandidateStateReader(sm).FrozenNativeBucket(windowH1, staked)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)
	_, err = newCandidateStateReader(sm).FrozenNativeBucket(windowH1, postFreeze)
	r.ErrorIs(err, eracow.ErrBucketPostFreeze)

	// --- era 2 freezes before era 1's drain finished -------------------------
	r.NoError(BeginEraCOWWindow(ctxAtH2, sm, h2))

	// This is the exact read runVoterDistributionChunk performs, and the exact
	// check it makes. Both succeed, so the era-1 drain proceeds.
	live, err := LoadEraCOWWindow(sm)
	r.NoError(err)
	r.True(live.Open(), "the drain's only window gate still passes")
	r.Equal(h2, live.FreezeHeight, "but it is no longer era 1's window")
	r.NotEqual(windowH1.FreezeHeight, live.FreezeHeight,
		"nothing reconciles the live window against the cursor's freeze height")

	// Corruption 1: the era-1 settlement now reads the post-H1 value. The work
	// item still says FreezeHeight == h1, so no other layer can notice.
	frozen, err = newCandidateStateReader(sm).FrozenNativeBucket(live, staked)
	r.NoError(err)
	r.Equal(big.NewInt(999), frozen.StakedAmount,
		"era 1 pays on the bucket's post-H1 amount")

	// Corruption 2: the high-water mark moved with the window, so a bucket
	// minted after era 1's boundary is admitted into era 1's payout.
	frozen, err = newCandidateStateReader(sm).FrozenNativeBucket(live, postFreeze)
	r.NoError(err)
	r.Equal(big.NewInt(700), frozen.StakedAmount,
		"a bucket that did not exist at h1 is payable in era 1")

	// The voter index goes the same way: a voter who first staked after h1 is
	// enumerated as an era-1 voter.
	newcomer := identityset.Address(3)
	indices, err := newCandidateStateReader(sm).FrozenNativeBucketIndices(windowH1, newcomer)
	r.NoError(err)
	r.Empty(indices, "correct answer for era 1")
	indices, err = newCandidateStateReader(sm).FrozenNativeBucketIndices(live, newcomer)
	r.NoError(err)
	r.Equal(BucketIndices{postFreeze}, indices, "answer the drain actually gets")

	// Era 1's copies were never deleted -- Begin queued them for GC rather than
	// dropping them -- so the drain had the right data available and read the
	// wrong tag. A guard, not a rescue, is what is missing.
	frozen, err = newCandidateStateReader(sm).FrozenNativeBucket(windowH1, staked)
	r.NoError(err)
	r.Equal(big.NewInt(100), frozen.StakedAmount)
}

// eraTestRawValue accepts any stored value, so the counters above can walk a
// namespace holding several unrelated types.
type eraTestRawValue struct{ data []byte }

func (v *eraTestRawValue) Serialize() ([]byte, error) { return v.data, nil }
func (v *eraTestRawValue) Deserialize(b []byte) error { v.data = append(v.data[:0], b...); return nil }
