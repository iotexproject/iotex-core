// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
)

func newTestTrie(t *testing.T) trie.Trie {
	t.Helper()
	tr, err := mptrie.New(
		mptrie.KVStoreOption(trie.NewMemKVStore()),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
	)
	require.NoError(t, err)
	require.NoError(t, tr.Start(context.Background()))
	return tr
}

func testHashFunc(data []byte) []byte {
	h := hash.Hash256b(data)
	return h[:]
}

func TestWitnessTrie_FirstTouchTracking(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	// Pre-populate some keys
	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))
	require.NoError(inner.Upsert(_k2b[:], _v2b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Read k1 — should record first touch with prestate value
	v, err := wt.Get(_k1b[:])
	require.NoError(err)
	require.Equal(_v1b[:], v)
	require.True(wt.hasFirstTouch[_k1b])
	require.Equal(_v1b[:], wt.firstTouch[_k1b])

	// Read k1 again — should NOT update first touch
	v, err = wt.Get(_k1b[:])
	require.NoError(err)
	require.Equal(_v1b[:], v)
	require.Equal(_v1b[:], wt.firstTouch[_k1b])

	// Read k3 (absent) — should record nil first touch
	_, err = wt.Get(_k3b[:])
	require.Error(err) // key not found
	require.True(wt.hasFirstTouch[_k3b])
	require.Nil(wt.firstTouch[_k3b])
}

func TestWitnessTrie_MutationCapturesPrestate(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// No prestate clone yet
	require.Nil(wt.prestate)

	// Upsert (mutation) should trigger prestate clone
	require.NoError(wt.Upsert(_k1b[:], _v2b[:]))
	require.NotNil(wt.prestate)

	// First touch should record the original prestate value
	require.True(wt.hasFirstTouch[_k1b])
	require.Equal(_v1b[:], wt.firstTouch[_k1b])

	// The inner trie should have the new value
	v, err := inner.Get(_k1b[:])
	require.NoError(err)
	require.Equal(_v2b[:], v)

	// The prestate trie should still have the original value
	v, err = wt.prestate.Get(_k1b[:])
	require.NoError(err)
	require.Equal(_v1b[:], v)
}

func TestWitnessTrie_DeleteCapturesPrestate(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Delete should trigger prestate clone and record first touch
	require.NoError(wt.Delete(_k1b[:]))
	require.NotNil(wt.prestate)
	require.True(wt.hasFirstTouch[_k1b])
	require.Equal(_v1b[:], wt.firstTouch[_k1b])

	// The prestate should still have the deleted value
	v, err := wt.prestate.Get(_k1b[:])
	require.NoError(err)
	require.Equal(_v1b[:], v)
}

func TestWitnessTrie_ReadOnlyNoPrestateClone(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Read-only access should NOT clone prestate
	_, err := wt.Get(_k1b[:])
	require.NoError(err)
	require.Nil(wt.prestate)
}

func TestWitnessTrie_BuildWitnessEmpty(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)
	wt := newWitnessTrie(inner, testHashFunc)

	// No accesses — BuildWitness returns nil
	w, err := wt.BuildWitness()
	require.NoError(err)
	require.Nil(w)
}

func TestWitnessTrie_BuildWitnessReadOnly(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))
	require.NoError(inner.Upsert(_k2b[:], _v2b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Read two keys
	_, err := wt.Get(_k1b[:])
	require.NoError(err)
	_, err = wt.Get(_k2b[:])
	require.NoError(err)

	// Build witness — should use current trie as proof source (no mutations)
	w, err := wt.BuildWitness()
	require.NoError(err)
	require.NotNil(w)
	require.Equal(2, len(w.Entries))
	require.NotEmpty(w.ProofNodes)

	// Storage root should match current trie root
	rootHash, err := inner.RootHash()
	require.NoError(err)
	require.Equal(hash.BytesToHash256(rootHash), w.StorageRoot)

	// Each entry should have the prestate value
	entryMap := make(map[hash.Hash256][]byte)
	for _, e := range w.Entries {
		entryMap[e.Key] = e.Value
	}
	require.Equal(_v1b[:], entryMap[_k1b])
	require.Equal(_v2b[:], entryMap[_k2b])
}

func TestWitnessTrie_BuildWitnessWithMutations(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))
	require.NoError(inner.Upsert(_k2b[:], _v2b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Read k1, then mutate k2
	_, err := wt.Get(_k1b[:])
	require.NoError(err)
	require.NoError(wt.Upsert(_k2b[:], _v3b[:]))

	// Build witness — proofs should come from prestate clone
	w, err := wt.BuildWitness()
	require.NoError(err)
	require.NotNil(w)
	require.Equal(2, len(w.Entries))
	require.NotEmpty(w.ProofNodes)

	// Storage root should be the PREstate root (before mutation)
	prestateRoot, err := wt.prestate.RootHash()
	require.NoError(err)
	require.Equal(hash.BytesToHash256(prestateRoot), w.StorageRoot)

	// Entries should have prestate values
	entryMap := make(map[hash.Hash256][]byte)
	for _, e := range w.Entries {
		entryMap[e.Key] = e.Value
	}
	require.Equal(_v1b[:], entryMap[_k1b])
	require.Equal(_v2b[:], entryMap[_k2b]) // prestate value, not the mutated v3b
}

func TestWitnessTrie_FirstTouchDeduplication(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Read k1, get prestate v1b
	_, err := wt.Get(_k1b[:])
	require.NoError(err)

	// Mutate k1 to v2b
	require.NoError(wt.Upsert(_k1b[:], _v2b[:]))

	// Read k1 again — should NOT update firstTouch (still v1b)
	v, err := wt.Get(_k1b[:])
	require.NoError(err)
	require.Equal(_v2b[:], v) // reads current value from inner
	require.Equal(_v1b[:], wt.firstTouch[_k1b]) // but firstTouch is still v1b

	// Build witness — entry should have original prestate value
	w, err := wt.BuildWitness()
	require.NoError(err)
	require.Equal(1, len(w.Entries))
	require.Equal(_v1b[:], w.Entries[0].Value)
}

func TestWitnessTrie_WriteOnlyKey(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	wt := newWitnessTrie(inner, testHashFunc)

	// Write to a new key (never existed in prestate)
	require.NoError(wt.Upsert(_k1b[:], _v1b[:]))

	// First touch should record nil (absent in prestate)
	require.True(wt.hasFirstTouch[_k1b])
	require.Nil(wt.firstTouch[_k1b])

	// Build witness
	w, err := wt.BuildWitness()
	require.NoError(err)
	require.NotNil(w)
	require.Equal(1, len(w.Entries))
	require.Equal(_k1b, w.Entries[0].Key)
	require.Nil(w.Entries[0].Value) // absent in prestate
}

func TestWitnessTrie_AbsentKeyProof(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	// Populate some keys so the trie isn't empty
	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Read an absent key
	_, err := wt.Get(_k2b[:])
	require.Error(err) // key not found

	// Build witness — should still have proof nodes for the absent key
	w, err := wt.BuildWitness()
	require.NoError(err)
	require.NotNil(w)
	require.Equal(1, len(w.Entries))
	require.Nil(w.Entries[0].Value)
	// Proof nodes should be non-empty (proves non-existence)
	require.NotEmpty(w.ProofNodes)
}

func TestWitnessTrie_ProofDeduplication(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	// Insert keys that will share trie path nodes
	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))
	require.NoError(inner.Upsert(_k2b[:], _v2b[:]))
	require.NoError(inner.Upsert(_k3b[:], _v3b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Access all keys
	_, err := wt.Get(_k1b[:])
	require.NoError(err)
	_, err = wt.Get(_k2b[:])
	require.NoError(err)
	_, err = wt.Get(_k3b[:])
	require.NoError(err)

	w, err := wt.BuildWitness()
	require.NoError(err)
	require.NotNil(w)
	require.Equal(3, len(w.Entries))

	// Proof nodes should be deduplicated — count of unique nodes
	// should be less than 3x individual proof counts
	proofNodeHashes := make(map[string]struct{})
	for _, node := range w.ProofNodes {
		h := testHashFunc(node)
		proofNodeHashes[string(h)] = struct{}{}
	}
	require.Equal(len(w.ProofNodes), len(proofNodeHashes))
}

func TestWitnessTrie_DelegatesTrieMethods(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// RootHash delegates
	expected, err := inner.RootHash()
	require.NoError(err)
	actual, err := wt.RootHash()
	require.NoError(err)
	require.Equal(expected, actual)

	// IsEmpty delegates
	require.Equal(inner.IsEmpty(), wt.IsEmpty())

	// SetRootHash delegates
	require.NoError(wt.SetRootHash(expected))
	actual2, err := wt.RootHash()
	require.NoError(err)
	require.Equal(expected, actual2)
}

func TestWitnessTrie_RevertSafety(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))
	require.NoError(inner.Upsert(_k2b[:], _v2b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Simulate a subcall: read k1 and mutate k2
	_, err := wt.Get(_k1b[:])
	require.NoError(err)
	require.NoError(wt.Upsert(_k2b[:], _v3b[:]))

	// k1 and k2 are now in firstTouch
	require.True(wt.hasFirstTouch[_k1b])
	require.True(wt.hasFirstTouch[_k2b])

	// In the real system, revert restores the underlying KVStore via
	// sm.Revert() + contract.LoadRoot(). The witnessTrie is shared
	// across Contract.Snapshot() copies (same pointer), so first-touch
	// data naturally survives revert. Verify this property:
	// even after the subcall is "reverted", first-touch data is preserved
	// because the stateless verifier needs it to re-execute.
	require.Equal(_v1b[:], wt.firstTouch[_k1b])
	require.Equal(_v2b[:], wt.firstTouch[_k2b])

	// Prestate clone also survives — its root represents pre-mutation state
	require.NotNil(wt.prestate)
	prestateRoot, err := wt.prestate.RootHash()
	require.NoError(err)
	require.NotEmpty(prestateRoot)

	// New accesses after "revert" are tracked correctly
	_, err = wt.Get(_k3b[:])
	require.Error(err) // k3 doesn't exist
	require.True(wt.hasFirstTouch[_k3b])

	// Build witness — should include all 3 keys
	w, err := wt.BuildWitness()
	require.NoError(err)
	require.NotNil(w)
	require.Equal(3, len(w.Entries))

	// Witness root should be the prestate root (before any mutation)
	require.Equal(hash.BytesToHash256(prestateRoot), w.StorageRoot)
}

func TestWitnessTrie_MultipleUpsertsSameKey(t *testing.T) {
	require := require.New(t)
	inner := newTestTrie(t)

	require.NoError(inner.Upsert(_k1b[:], _v1b[:]))

	wt := newWitnessTrie(inner, testHashFunc)

	// Multiple mutations to same key
	require.NoError(wt.Upsert(_k1b[:], _v2b[:]))
	require.NoError(wt.Upsert(_k1b[:], _v3b[:]))
	require.NoError(wt.Upsert(_k1b[:], _v4b[:]))

	// First touch should be the original prestate value
	require.Equal(_v1b[:], wt.firstTouch[_k1b])

	// Build — only 1 entry
	w, err := wt.BuildWitness()
	require.NoError(err)
	require.Equal(1, len(w.Entries))
	require.Equal(_v1b[:], w.Entries[0].Value)
}
