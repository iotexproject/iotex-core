// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
)

var _testAddr = common.BytesToAddress([]byte("testcontract12345678"))

func _testAddrHash160() hash.Hash160 {
	return hash.BytesToHash160(_testAddr.Bytes())
}

func _testHashFuncAddr() func([]byte) []byte {
	return func(data []byte) []byte {
		h := hash.Hash256b(append(_testAddr.Bytes(), data...))
		return h[:]
	}
}

// buildWitness is a test helper that creates a witnessTrie, performs reads
// and writes, then returns the witness and the producer's post-state root hash.
// It uses a recordingKVStore to mirror the production code path where
// execution-time node loads are recorded for inclusion in the witness.
func buildWitness(t *testing.T, prestateKVs map[hash.Hash256][]byte, reads []hash.Hash256, mutations map[hash.Hash256][]byte) (*ContractStorageWitness, []byte) {
	t.Helper()
	require := require.New(t)

	// Create a MemKVStore-backed inner trie, then reload it from kvstore so
	// that internal nodes become hashNodes (lazy references). This mirrors the
	// production path where the contract's trie is loaded from persistent
	// storage and children are only loaded on demand through the recordingKVStore.
	hashFunc := _testHashFuncAddr()
	baseKV := trie.NewMemKVStore()
	baseTrie, err := mptrie.New(
		mptrie.KVStoreOption(baseKV),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(hashFunc),
	)
	require.NoError(err)
	require.NoError(baseTrie.Start(nil))

	for k, v := range prestateKVs {
		require.NoError(baseTrie.Upsert(k[:], v))
	}

	// Reload the trie from kvstore so all internal nodes are hashNodes.
	// This matches production behavior where nodes are loaded lazily.
	rootHash, err := baseTrie.RootHash()
	require.NoError(err)
	reloadedTrie, err := mptrie.New(
		mptrie.KVStoreOption(baseKV),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(hashFunc),
		mptrie.RootHashOption(rootHash),
	)
	require.NoError(err)
	require.NoError(reloadedTrie.Start(nil))

	// Clone through recordingKVStore, mirroring production path.
	recKV := newRecordingKVStore(baseKV)
	clonedTrie, err := reloadedTrie.Clone(recKV)
	require.NoError(err)
	// Prime the root node so it's included in the recorded set.
	clonedRoot, err := clonedTrie.RootHash()
	require.NoError(err)
	if len(clonedRoot) > 0 {
		require.NoError(recKV.prime(clonedRoot))
	}
	wt := newWitnessTrie(clonedTrie, hashFunc, recKV)

	// Record reads
	for _, k := range reads {
		wt.Get(k[:]) // ignore error (absent keys)
	}

	// Apply mutations
	for k, v := range mutations {
		if v == nil {
			require.NoError(wt.Delete(k[:]))
		} else {
			require.NoError(wt.Upsert(k[:], v))
		}
	}

	// Get producer's post-state root
	postRoot, err := clonedTrie.RootHash()
	require.NoError(err)

	w, err := wt.BuildWitness()
	require.NoError(err)
	require.NotNil(w)
	require.NoError(VerifyContractStorageWitness(_testAddr, w))
	return w, postRoot
}

// TestStatelessTrie_ReadPrestate verifies that a stateless MPT built from
// witness proof nodes returns correct prestate values.
func TestStatelessTrie_ReadPrestate(t *testing.T) {
	require := require.New(t)

	prestate := map[hash.Hash256][]byte{
		_k1b: _v1b[:],
		_k2b: _v2b[:],
	}
	reads := []hash.Hash256{_k1b, _k2b, _k3b}
	w, _ := buildWitness(t, prestate, reads, nil)

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil, false)
	require.NoError(err)

	// Existing keys return correct values
	v, err := tr.Get(_k1b[:])
	require.NoError(err)
	require.Equal(_v1b[:], v)

	v, err = tr.Get(_k2b[:])
	require.NoError(err)
	require.Equal(_v2b[:], v)

	// Absent key returns ErrNotExist
	_, err = tr.Get(_k3b[:])
	require.ErrorIs(err, trie.ErrNotExist)
}

// TestStatelessTrie_MutateAndRootHash verifies the key property: after applying
// the same mutations, the stateless MPT's RootHash matches the producer's
// post-state root. This is the fix for the Account.Root persistence bug.
func TestStatelessTrie_MutateAndRootHash(t *testing.T) {
	require := require.New(t)

	prestate := map[hash.Hash256][]byte{
		_k1b: _v1b[:],
		_k2b: _v2b[:],
	}
	mutations := map[hash.Hash256][]byte{
		_k2b: _v3b[:], // overwrite k2
	}
	reads := []hash.Hash256{_k1b, _k2b}
	w, producerPostRoot := buildWitness(t, prestate, reads, mutations)

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil, false)
	require.NoError(err)

	// Before mutation: root equals prestate root
	rh, err := tr.RootHash()
	require.NoError(err)
	require.Equal(w.StorageRoot[:], rh)

	// Apply the same mutation on the stateless side
	require.NoError(tr.Upsert(_k2b[:], _v3b[:]))

	// After mutation: root equals producer's post-state root
	rh, err = tr.RootHash()
	require.NoError(err)
	require.Equal(producerPostRoot, rh, "stateless MPT post-state root must match producer's")
}

// TestStatelessTrie_DeleteAndRootHash verifies that deleting a key on the
// stateless MPT produces the correct post-state root.
func TestStatelessTrie_DeleteAndRootHash(t *testing.T) {
	require := require.New(t)

	prestate := map[hash.Hash256][]byte{
		_k1b: _v1b[:],
		_k2b: _v2b[:],
	}
	mutations := map[hash.Hash256][]byte{
		_k2b: nil, // delete k2
	}
	reads := []hash.Hash256{_k1b, _k2b}
	w, producerPostRoot := buildWitness(t, prestate, reads, mutations)

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil, false)
	require.NoError(err)

	require.NoError(tr.Delete(_k2b[:]))

	rh, err := tr.RootHash()
	require.NoError(err)
	require.Equal(producerPostRoot, rh)

	// Deleted key is gone
	_, err = tr.Get(_k2b[:])
	require.ErrorIs(err, trie.ErrNotExist)

	// Other key is still readable
	v, err := tr.Get(_k1b[:])
	require.NoError(err)
	require.Equal(_v1b[:], v)
}

// TestStatelessTrie_EmptyPrestate verifies that an empty-storage contract
// (StorageRoot == ZeroHash256) works correctly: upserts produce the right root.
func TestStatelessTrie_EmptyPrestate(t *testing.T) {
	require := require.New(t)

	// Producer: empty trie, insert k1
	inner := newTestTrieWithAddr(t, _testAddr)
	wt := newWitnessTrie(inner, _testHashFuncAddr(), nil)
	_, err := wt.Get(_k1b[:]) // record first touch (absent)
	require.Error(err)
	require.NoError(wt.Upsert(_k1b[:], _v1b[:]))

	postRoot, err := inner.RootHash()
	require.NoError(err)

	w, err := wt.BuildWitness()
	require.NoError(err)
	require.Equal(hash.ZeroHash256, w.StorageRoot)

	// Validator: replay on stateless MPT
	tr, err := newStatelessTrie(_testAddrHash160(), w, nil, false)
	require.NoError(err)

	require.NoError(tr.Upsert(_k1b[:], _v1b[:]))

	rh, err := tr.RootHash()
	require.NoError(err)
	require.Equal(postRoot, rh)
}

// TestStatelessTrie_InsertNewKey verifies that inserting a key that didn't exist
// in prestate produces the correct root hash.
func TestStatelessTrie_InsertNewKey(t *testing.T) {
	require := require.New(t)

	prestate := map[hash.Hash256][]byte{
		_k1b: _v1b[:],
	}
	mutations := map[hash.Hash256][]byte{
		_k3b: _v3b[:], // insert new key
	}
	reads := []hash.Hash256{_k1b, _k3b}
	w, producerPostRoot := buildWitness(t, prestate, reads, mutations)

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil, false)
	require.NoError(err)

	require.NoError(tr.Upsert(_k3b[:], _v3b[:]))

	rh, err := tr.RootHash()
	require.NoError(err)
	require.Equal(producerPostRoot, rh)
}

// TestStatelessTrie_MultipleMutations verifies multiple mutations produce the
// correct post-state root.
func TestStatelessTrie_MultipleMutations(t *testing.T) {
	require := require.New(t)

	prestate := map[hash.Hash256][]byte{
		_k1b: _v1b[:],
		_k2b: _v2b[:],
	}
	mutations := map[hash.Hash256][]byte{
		_k1b: _v3b[:], // overwrite k1
		_k2b: nil,     // delete k2
		_k3b: _v4b[:], // insert k3
	}
	reads := []hash.Hash256{_k1b, _k2b, _k3b}
	w, producerPostRoot := buildWitness(t, prestate, reads, mutations)

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil, false)
	require.NoError(err)

	require.NoError(tr.Upsert(_k1b[:], _v3b[:]))
	require.NoError(tr.Delete(_k2b[:]))
	require.NoError(tr.Upsert(_k3b[:], _v4b[:]))

	rh, err := tr.RootHash()
	require.NoError(err)
	require.Equal(producerPostRoot, rh)
}

// TestStatelessTrie_NoWitnessEmptyTrie verifies the case where a contract has
// no witness (only touched for balance/nonce), producing an empty trie.
func TestStatelessTrie_NoWitnessEmptyTrie(t *testing.T) {
	require := require.New(t)

	tr, err := newStatelessTrie(_testAddrHash160(), &ContractStorageWitness{
		StorageRoot: hash.ZeroHash256,
	}, nil, false)
	require.NoError(err)
	require.True(tr.IsEmpty())

	_, err = tr.Get(_k1b[:])
	require.ErrorIs(err, trie.ErrNotExist)
}

// newTestTrieWithAddr creates an MPT trie with the contract-scoped hash function.
func newTestTrieWithAddr(t *testing.T, addr common.Address) trie.Trie {
	t.Helper()
	tr, err := mptrie.New(
		mptrie.KVStoreOption(trie.NewMemKVStore()),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(func(data []byte) []byte {
			h := hash.Hash256b(append(addr.Bytes(), data...))
			return h[:]
		}),
	)
	require.NoError(t, err)
	require.NoError(t, tr.Start(nil))
	return tr
}

// TestStatelessTrie_DeleteBranchCollapseWithUnreadSibling exercises the case
// where Delete triggers a branch collapse and the sibling node was never read
// (not in firstTouch). The recording kvstore captures the sibling's node load
// during the producer's Delete, so the validator can replay the Delete.
//
// Key structure: keyA and keyB share the first byte (creating a sub-branch
// under the root), keyC has a different first byte. The sub-branch has exactly
// 2 children. Deleting keyA collapses it, loading keyB as the orphan sibling.
func TestStatelessTrie_DeleteBranchCollapseWithUnreadSibling(t *testing.T) {
	require := require.New(t)

	// keyA[0]==keyB[0]==0x00 means both go through the same root child (extension+branch).
	// They differ at byte 1, creating a 2-child sub-branch that collapses on delete.
	var keyA, keyB, keyC hash.Hash256
	keyA[0] = 0x00
	keyA[1] = 0x01 // sub-branch child 0x01
	keyB[0] = 0x00
	keyB[1] = 0x02 // sub-branch child 0x02
	keyC[0] = 0x10 // different first byte — separate root child

	valA := []byte("valueA")
	valB := []byte("valueB")
	valC := []byte("valueC")

	prestate := map[hash.Hash256][]byte{
		keyA: valA,
		keyB: valB,
		keyC: valC,
	}
	// Only read keyA (to be deleted) and keyC. Do NOT read keyB.
	// Delete(keyA) triggers sub-branch collapse, which loads keyB's sibling
	// node via the recordingKVStore.
	reads := []hash.Hash256{keyA, keyC}
	mutations := map[hash.Hash256][]byte{
		keyA: nil, // delete keyA → triggers branch collapse needing keyB
	}
	w, producerPostRoot := buildWitness(t, prestate, reads, mutations)

	t.Logf("Proof has %d nodes, entries=%d", len(w.ProofNodes), len(w.Entries))

	// Validate: the stateless trie can replay Delete(keyA)
	tr, err := newStatelessTrie(_testAddrHash160(), w, nil, false)
	require.NoError(err)

	err = tr.Delete(keyA[:])
	require.NoError(err)

	// Post-state root must match the producer's
	rh, err := tr.RootHash()
	require.NoError(err)
	require.Equal(producerPostRoot, rh)

	// keyC should still be readable (was in the reads set)
	v, err := tr.Get(keyC[:])
	require.NoError(err)
	require.Equal(valC, v)

	// keyA should be gone
	_, err = tr.Get(keyA[:])
	require.ErrorIs(err, trie.ErrNotExist)
}

// TestStatelessTrie_DeleteBranchCollapseFollowedByGet exercises the scenario
// from production: Delete causes branch collapse, then a subsequent Get on a
// different key (that was read before the Delete) traverses the restructured
// trie and needs proof nodes for the path.
func TestStatelessTrie_DeleteBranchCollapseFollowedByGet(t *testing.T) {
	require := require.New(t)

	// Same key structure: keyA + keyB share first nibble, keyC is separate
	var keyA, keyB, keyC hash.Hash256
	keyA[0] = 0x00
	keyA[1] = 0xAA
	keyB[0] = 0x01
	keyB[1] = 0xBB
	keyC[0] = 0x10
	keyC[1] = 0xCC

	prestate := map[hash.Hash256][]byte{
		keyA: []byte("A"),
		keyB: []byte("B"),
		keyC: []byte("C"),
	}
	// Read all keys, but keyB is only read (no mutation)
	reads := []hash.Hash256{keyA, keyB, keyC}
	mutations := map[hash.Hash256][]byte{
		keyA: nil, // delete
	}
	w, producerPostRoot := buildWitness(t, prestate, reads, mutations)

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil, false)
	require.NoError(err)

	// Read keyB first (like the producer did)
	v, err := tr.Get(keyB[:])
	require.NoError(err)
	require.Equal([]byte("B"), v)

	// Delete keyA
	require.NoError(tr.Delete(keyA[:]))

	// Read keyB again after the structural change
	v, err = tr.Get(keyB[:])
	require.NoError(err)
	require.Equal([]byte("B"), v)

	rh, err := tr.RootHash()
	require.NoError(err)
	require.Equal(producerPostRoot, rh)
}
