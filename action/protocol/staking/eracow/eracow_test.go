// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package eracow

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// forkCtx builds a context whose FeatureCtx has IIP-59 either off or on, the
// same way the contractstaking owner-index tests do.
func forkCtx(activated bool) context.Context {
	const height = uint64(1)
	g := genesis.TestDefault()
	if activated {
		g.ToBeEnabledBlockHeight = height
	}
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	return protocol.WithFeatureCtx(ctx)
}

func newTestSM(t *testing.T) *mock_chainmanager.MockStateManager {
	return testdb.NewMockStateManager(gomock.NewController(t))
}

// testValue is a stand-in for whatever typed value a hook site copies aside.
type testValue struct{ v []byte }

func (t *testValue) Serialize() ([]byte, error) { return append([]byte{}, t.v...), nil }
func (t *testValue) Deserialize(b []byte) error { t.v = append([]byte{}, b...); return nil }

func val(s string) *testValue { return &testValue{v: []byte(s)} }

const liveNS = "live"

// putLive writes a value at the covered key's own (live) location.
func putLive(t *testing.T, sm protocol.StateManager, key []byte, v *testValue) {
	t.Helper()
	_, err := sm.PutState(v, protocol.NamespaceOption(liveNS), protocol.KeyOption(key))
	require.NoError(t, err)
}

func TestControlSerializeRoundTrip(t *testing.T) {
	r := require.New(t)
	c := &Control{
		FreezeHeight:     1234,
		TotalBucketCount: 99,
		NextSeq:          7,
		Pending: []GCState{
			{FreezeHeight: 100, Cursor: 3, End: 40},
			{FreezeHeight: 200, Cursor: 0, End: 1},
		},
		ContractCounts: []ContractBucketCount{
			{Contract: identityset.Address(1).Bytes(), NumOfBuckets: 11},
			{Contract: identityset.Address(2).Bytes(), NumOfBuckets: 0},
		},
	}
	data, err := c.Serialize()
	r.NoError(err)
	var got Control
	r.NoError(got.Deserialize(data))
	r.Equal(*c, got)

	// The Erigon container round-trips through the same bytes.
	gv, err := c.Encode()
	r.NoError(err)
	var viaErigon Control
	r.NoError(viaErigon.Decode(gv))
	r.Equal(*c, viaErigon)

	// A truncated body is rejected rather than silently reinterpreted.
	r.Error(got.Deserialize(data[:len(data)-1]))
	r.Error(got.Deserialize(data[:3]))

	// A contract address of the wrong width never makes it into state.
	bad := &Control{ContractCounts: []ContractBucketCount{{Contract: []byte{1, 2, 3}}}}
	_, err = bad.Serialize()
	r.Error(err)
}

func TestEntryRoundTripAndTombstone(t *testing.T) {
	r := require.New(t)
	e := &Entry{Exists: true, Data: []byte("payload")}
	data, err := e.Serialize()
	r.NoError(err)
	var got Entry
	r.NoError(got.Deserialize(data))
	r.Equal(*e, got)

	tomb := &Entry{}
	data, err = tomb.Serialize()
	r.NoError(err)
	r.Len(data, 1)
	var gotTomb Entry
	r.NoError(gotTomb.Deserialize(data))
	r.False(gotTomb.Exists)
	r.Empty(gotTomb.Data)

	r.Error(got.Deserialize(nil))
}

// TestFirstWriteWins is the core invariant: the copy is the value at H, not the
// value at the most recent mutation.
func TestFirstWriteWins(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)
	r.NoError(Begin(ctx, sm, 500, 10, nil))

	s := NewSession(ctx, sm)
	sub := NativeBucketSubkey(3)

	// Mutation 1: bucket held "at-H" beforehand.
	r.NoError(s.Snapshot(KindNativeBucket, sub, val("at-H")))
	// Mutations 2 and 3, later in the era, must not clobber the copy.
	r.NoError(s.Snapshot(KindNativeBucket, sub, val("later")))
	r.NoError(s.Snapshot(KindNativeBucket, sub, val("even-later")))
	// A fresh session in a later block behaves identically.
	r.NoError(NewSession(ctx, sm).Snapshot(KindNativeBucket, sub, val("next-block")))

	var got testValue
	r.NoError(Resolve(sm, 500, KindNativeBucket, sub, liveNS, []byte("whatever"), &got))
	r.Equal("at-H", string(got.v))

	// Exactly one journal record was appended, so GC will not double-count.
	c, err := readControl(sm)
	r.NoError(err)
	r.EqualValues(1, c.NextSeq)
}

// TestFrozenReadResolution covers the three outcomes of Resolve.
func TestFrozenReadResolution(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)
	r.NoError(Begin(ctx, sm, 500, 10, nil))
	s := NewSession(ctx, sm)

	// (1) Copied key: the copy wins over the (now different) live value.
	copied := NativeBucketSubkey(1)
	copiedLiveKey := []byte("live-1")
	putLive(t, sm, copiedLiveKey, val("mutated-after-H"))
	r.NoError(s.Snapshot(KindNativeBucket, copied, val("as-of-H")))

	var got testValue
	r.NoError(Resolve(sm, 500, KindNativeBucket, copied, liveNS, copiedLiveKey, &got))
	r.Equal("as-of-H", string(got.v))

	// (2) Uncopied key: never mutated since H, so the live value is the H value.
	untouched := NativeBucketSubkey(2)
	untouchedLiveKey := []byte("live-2")
	putLive(t, sm, untouchedLiveKey, val("unchanged-since-H"))
	got = testValue{}
	r.NoError(Resolve(sm, 500, KindNativeBucket, untouched, liveNS, untouchedLiveKey, &got))
	r.Equal("unchanged-since-H", string(got.v))

	// (3) Tombstone: created after H, so it did not exist at H.
	created := NativeBucketSubkey(3)
	createdLiveKey := []byte("live-3")
	r.NoError(s.Snapshot(KindNativeBucket, created, nil))
	putLive(t, sm, createdLiveKey, val("born-after-H"))
	got = testValue{}
	err := Resolve(sm, 500, KindNativeBucket, created, liveNS, createdLiveKey, &got)
	r.ErrorIs(err, ErrNotFrozen)

	// (4) Neither a copy nor a live value.
	err = Resolve(sm, 500, KindNativeBucket, NativeBucketSubkey(4), liveNS, []byte("nope"), &got)
	r.Equal(state.ErrStateNotExist, errors.Cause(err))

	// (5) A resolve against the wrong era tag does not see this era's copies.
	got = testValue{}
	r.NoError(Resolve(sm, 501, KindNativeBucket, copied, liveNS, copiedLiveKey, &got))
	r.Equal("mutated-after-H", string(got.v))

	// (6) Kind is part of the key, so kinds cannot collide on a shared subkey.
	got = testValue{}
	r.NoError(Resolve(sm, 500, KindLSDVoterIndex, copied, liveNS, copiedLiveKey, &got))
	r.Equal("mutated-after-H", string(got.v))

	r.Error(Resolve(sm, 0, KindNativeBucket, copied, liveNS, copiedLiveKey, &got))
}

// hostileSM fails the test on any state access at all. It is how the
// pre-activation "no new reads, no new writes" claim is enforced rather than
// merely asserted about the resulting state.
type hostileSM struct {
	t *testing.T
}

func (h *hostileSM) fail(op string) {
	h.t.Helper()
	h.t.Fatalf("pre-activation code touched state via %s", op)
}

func (h *hostileSM) Height() (uint64, error) { h.fail("Height"); return 0, nil }
func (h *hostileSM) State(interface{}, ...protocol.StateOption) (uint64, error) {
	h.fail("State")
	return 0, nil
}

func (h *hostileSM) States(...protocol.StateOption) (uint64, state.Iterator, error) {
	h.fail("States")
	return 0, nil, nil
}
func (h *hostileSM) ReadView(string) (protocol.View, error) { h.fail("ReadView"); return nil, nil }
func (h *hostileSM) Snapshot() int                          { h.fail("Snapshot"); return 0 }
func (h *hostileSM) Revert(int) error                       { h.fail("Revert"); return nil }
func (h *hostileSM) PutState(interface{}, ...protocol.StateOption) (uint64, error) {
	h.fail("PutState")
	return 0, nil
}

func (h *hostileSM) DelState(...protocol.StateOption) (uint64, error) {
	h.fail("DelState")
	return 0, nil
}
func (h *hostileSM) WriteView(string, protocol.View) error { h.fail("WriteView"); return nil }

// TestFeatureGateTouchesNothing pins the hard-fork-safety property: before
// IIP-59 activates, every entry point of this package is inert.
func TestFeatureGateTouchesNothing(t *testing.T) {
	r := require.New(t)
	ctx := forkCtx(false)
	hostile := &hostileSM{t: t}

	r.False(Enabled(ctx))
	r.NoError(Begin(ctx, hostile, 500, 10, []ContractBucketCount{{Contract: identityset.Address(1).Bytes(), NumOfBuckets: 5}}))
	r.NoError(Seal(ctx, hostile))
	n, err := CollectGarbage(ctx, hostile, 100)
	r.NoError(err)
	r.Zero(n)

	s := NewSession(ctx, hostile)
	active, err := s.Active()
	r.NoError(err)
	r.False(active)
	r.NoError(s.Snapshot(KindNativeBucket, NativeBucketSubkey(1), val("x")))
	h, err := s.FreezeHeight()
	r.NoError(err)
	r.Zero(h)

	// A context with no feature context at all also reads as pre-activation.
	r.False(Enabled(context.Background()))
	r.NoError(NewSession(context.Background(), hostile).Snapshot(KindNativeBucket, NativeBucketSubkey(1), val("x")))

	// And nothing at all landed in state: a real SM sees an empty control key.
	sm := newTestSM(t)
	r.NoError(Begin(ctx, sm, 500, 10, nil))
	c, err := readControl(sm)
	r.NoError(err)
	r.Nil(c)
	w, err := LoadWindow(sm)
	r.NoError(err)
	r.False(w.Open())
}

// TestNoOutstandingDrainIsANoOp covers the window between drain completion and
// the next era boundary, and the window before the first ever boundary.
func TestNoOutstandingDrainIsANoOp(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)

	// Before any Begin: activated, but nothing to protect.
	pre := NewSession(ctx, sm)
	active, err := pre.Active()
	r.NoError(err)
	r.False(active)
	r.NoError(pre.Snapshot(KindNativeBucket, NativeBucketSubkey(1), val("ignored")))
	c, err := readControl(sm)
	r.NoError(err)
	r.Nil(c)

	// Open, copy one thing, then seal.
	r.NoError(Begin(ctx, sm, 500, 10, nil))
	r.NoError(NewSession(ctx, sm).Snapshot(KindNativeBucket, NativeBucketSubkey(1), val("as-of-H")))
	r.NoError(Seal(ctx, sm))

	w, err := LoadWindow(sm)
	r.NoError(err)
	r.False(w.Open())

	// After Seal: sessions are inert again, and no journal grows.
	post := NewSession(ctx, sm)
	active, err = post.Active()
	r.NoError(err)
	r.False(active)
	r.NoError(post.Snapshot(KindNativeBucket, NativeBucketSubkey(2), val("ignored")))
	c, err = readControl(sm)
	r.NoError(err)
	r.EqualValues(0, c.NextSeq)
	r.Len(c.Pending, 1)
	r.EqualValues(1, c.Pending[0].End)

	// A session built before Seal must not write into the sealed era either:
	// its cached window is re-validated against the control record before the
	// first actual copy.
	stale := NewSession(ctx, sm)
	r.NoError(Begin(ctx, sm, 900, 10, nil))
	staleDuring := NewSession(ctx, sm)
	_, err = staleDuring.Active()
	r.NoError(err)
	r.NoError(Seal(ctx, sm))
	r.NoError(staleDuring.Snapshot(KindNativeBucket, NativeBucketSubkey(3), val("too-late")))
	_, err = sm.State(&Entry{},
		protocol.NamespaceOption(Namespace),
		protocol.KeyOption(EntryKey(900, KindNativeBucket, NativeBucketSubkey(3))),
	)
	r.Equal(state.ErrStateNotExist, errors.Cause(err))
	// A session loads the window on first use, not at construction, so this one
	// -- built before the second Begin but first used after the second Seal --
	// sees no window.
	staleActive, err := stale.Active()
	r.NoError(err)
	r.False(staleActive)
}

func TestBeginIsIdempotentAtTheSameHeight(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)

	r.NoError(Begin(ctx, sm, 500, 10, nil))
	r.NoError(NewSession(ctx, sm).Snapshot(KindNativeBucket, NativeBucketSubkey(1), val("as-of-H")))
	// A replayed boundary must not reset the sequence counter and orphan the
	// entry already written under it.
	r.NoError(Begin(ctx, sm, 500, 10, nil))
	c, err := readControl(sm)
	r.NoError(err)
	r.EqualValues(1, c.NextSeq)
	r.Empty(c.Pending)

	r.Error(Begin(ctx, sm, 0, 0, nil))
}

func TestBeginOverAnOpenWindowQueuesTheOldEra(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)

	r.NoError(Begin(ctx, sm, 500, 10, nil))
	r.NoError(NewSession(ctx, sm).Snapshot(KindNativeBucket, NativeBucketSubkey(1), val("a")))
	r.NoError(NewSession(ctx, sm).Snapshot(KindNativeBucket, NativeBucketSubkey(2), val("b")))
	r.NoError(Begin(ctx, sm, 900, 20, nil))

	c, err := readControl(sm)
	r.NoError(err)
	r.EqualValues(900, c.FreezeHeight)
	r.EqualValues(20, c.TotalBucketCount)
	r.EqualValues(0, c.NextSeq)
	r.Len(c.Pending, 1)
	r.Equal(GCState{FreezeHeight: 500, Cursor: 0, End: 2}, c.Pending[0])
}

// TestBucketHighWaterMarks pins both HWM boundaries, including the deliberate
// off-by-one between them.
func TestBucketHighWaterMarks(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)

	contractA := identityset.Address(1).Bytes()
	contractB := identityset.Address(2).Bytes()
	unknown := identityset.Address(3).Bytes()
	r.NoError(Begin(ctx, sm, 500, 10, []ContractBucketCount{
		{Contract: contractA, NumOfBuckets: 7},
		{Contract: contractB, NumOfBuckets: 0},
	}))

	w, err := LoadWindow(sm)
	r.NoError(err)
	r.True(w.Open())
	r.EqualValues(500, w.FreezeHeight)

	// Native: totalBucketCount is the NEXT index, so the bound is exclusive.
	r.True(w.NativeBucketExisted(0))
	r.True(w.NativeBucketExisted(9))
	r.False(w.NativeBucketExisted(10))
	r.False(w.NativeBucketExisted(11))

	// LSD: NumOfBuckets is the MAX SEEN id, so the bound is inclusive.
	r.True(w.ContractBucketExisted(contractA, 0))
	r.True(w.ContractBucketExisted(contractA, 7))
	r.False(w.ContractBucketExisted(contractA, 8))
	// A contract that had minted exactly one bucket (id 0) at H.
	r.True(w.ContractBucketExisted(contractB, 0))
	r.False(w.ContractBucketExisted(contractB, 1))
	// A contract with no frozen record had nothing at H.
	r.False(w.ContractBucketExisted(unknown, 0))

	// A closed window rejects everything, which keeps a missing Begin from
	// silently admitting post-H buckets.
	var closed Window
	r.False(closed.NativeBucketExisted(0))
	r.False(closed.ContractBucketExisted(contractA, 0))
}

// TestGarbageCollectionIsBoundedAndComplete covers both halves of the GC
// contract: never more than max deletions in one call, and eventually every
// entry and journal record of a sealed era is gone.
func TestGarbageCollectionIsBoundedAndComplete(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)

	const total = 25
	r.NoError(Begin(ctx, sm, 500, 100, nil))
	for i := 0; i < total; i++ {
		r.NoError(NewSession(ctx, sm).Snapshot(KindNativeBucket, NativeBucketSubkey(uint64(i)), val("as-of-H")))
	}
	// Nothing is collectable while the drain is outstanding.
	n, err := CollectGarbage(ctx, sm, 100)
	r.NoError(err)
	r.Zero(n)
	pending, err := PendingGarbage(sm)
	r.NoError(err)
	r.Zero(pending)

	r.NoError(Seal(ctx, sm))
	pending, err = PendingGarbage(sm)
	r.NoError(err)
	r.EqualValues(total, pending)

	// Bounded: each call removes at most the chunk size.
	const chunk = 4
	rounds := 0
	for {
		n, err = CollectGarbage(ctx, sm, chunk)
		r.NoError(err)
		r.LessOrEqual(n, chunk)
		if n == 0 {
			break
		}
		rounds++
		r.Less(rounds, total+2)
	}
	r.Equal((total+chunk-1)/chunk, rounds)

	// Complete: every entry and journal record is gone.
	for i := 0; i < total; i++ {
		_, err = sm.State(&Entry{},
			protocol.NamespaceOption(Namespace),
			protocol.KeyOption(EntryKey(500, KindNativeBucket, NativeBucketSubkey(uint64(i)))),
		)
		r.Equal(state.ErrStateNotExist, errors.Cause(err), "entry %d survived GC", i)
		_, err = sm.State(&journalRecord{},
			protocol.NamespaceOption(Namespace),
			protocol.KeyOption(JournalKey(500, uint64(i))),
		)
		r.Equal(state.ErrStateNotExist, errors.Cause(err), "journal %d survived GC", i)
	}
	pending, err = PendingGarbage(sm)
	r.NoError(err)
	r.Zero(pending)

	// With nothing open and nothing pending, the control record is retired.
	c, err := readControl(sm)
	r.NoError(err)
	r.Nil(c)

	// max <= 0 collects nothing rather than everything.
	n, err = CollectGarbage(ctx, sm, 0)
	r.NoError(err)
	r.Zero(n)
}

// TestGarbageCollectionSpansEras checks the FIFO drains eras in order and does
// not lose one when a second is queued behind it.
func TestGarbageCollectionSpansEras(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)

	r.NoError(Begin(ctx, sm, 500, 10, nil))
	for i := 0; i < 3; i++ {
		r.NoError(NewSession(ctx, sm).Snapshot(KindNativeBucket, NativeBucketSubkey(uint64(i)), val("era-500")))
	}
	r.NoError(Begin(ctx, sm, 900, 10, nil))
	for i := 0; i < 2; i++ {
		r.NoError(NewSession(ctx, sm).Snapshot(KindLSDVoterIndex, AddrSubkey(identityset.Address(i).Bytes()), val("era-900")))
	}
	r.NoError(Seal(ctx, sm))

	pending, err := PendingGarbage(sm)
	r.NoError(err)
	r.EqualValues(5, pending)

	total := 0
	for {
		n, err := CollectGarbage(ctx, sm, 2)
		r.NoError(err)
		if n == 0 {
			break
		}
		total += n
	}
	r.Equal(5, total)
	c, err := readControl(sm)
	r.NoError(err)
	r.Nil(c)
}

// TestGarbageCollectionToleratesAMissingJournalRecord makes the replay path
// explicit: a hole in the sequence advances the cursor rather than stalling.
func TestGarbageCollectionToleratesAMissingJournalRecord(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	ctx := forkCtx(true)

	r.NoError(Begin(ctx, sm, 500, 10, nil))
	for i := 0; i < 3; i++ {
		r.NoError(NewSession(ctx, sm).Snapshot(KindNativeBucket, NativeBucketSubkey(uint64(i)), val("x")))
	}
	r.NoError(Seal(ctx, sm))

	_, err := sm.DelState(
		protocol.NamespaceOption(Namespace),
		protocol.KeyOption(JournalKey(500, 1)),
		protocol.ObjectOption(&journalRecord{}),
	)
	r.NoError(err)

	n, err := CollectGarbage(ctx, sm, 100)
	r.NoError(err)
	r.Equal(3, n)
	c, err := readControl(sm)
	r.NoError(err)
	r.Nil(c)
}

func TestKeyLayout(t *testing.T) {
	r := require.New(t)
	// The tag bytes are consensus-visible; pin them so a reordering of the
	// iota block in the staking package cannot silently move them.
	r.EqualValues(9, ControlPrefix)
	r.EqualValues(10, EntryPrefix)
	r.EqualValues(11, JournalPrefix)
	r.EqualValues(1, KindNativeBucket)
	r.EqualValues(2, KindNativeVoterIndex)
	r.EqualValues(3, KindLSDBucket)
	r.EqualValues(4, KindLSDVoterIndex)

	r.Equal([]byte{10, 0, 0, 0, 0, 0, 0, 1, 244, 1, 0, 0, 0, 0, 0, 0, 0, 5},
		EntryKey(500, KindNativeBucket, NativeBucketSubkey(5)))
	r.Equal([]byte{11, 0, 0, 0, 0, 0, 0, 1, 244, 0, 0, 0, 0, 0, 0, 0, 6},
		JournalKey(500, 6))

	// Distinct (era, kind, subkey) triples never collide.
	contract := identityset.Address(1).Bytes()
	r.NotEqual(EntryKey(500, KindLSDBucket, LSDBucketSubkey(contract, 1)),
		EntryKey(500, KindLSDBucket, LSDBucketSubkey(contract, 2)))
	r.NotEqual(EntryKey(500, KindLSDBucket, LSDBucketSubkey(contract, 1)),
		EntryKey(501, KindLSDBucket, LSDBucketSubkey(contract, 1)))
	r.Len(LSDBucketSubkey(contract, 1), 28)
	r.Len(AddrSubkey(contract), 20)
}
