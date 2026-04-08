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
func buildWitness(t *testing.T, prestateKVs map[hash.Hash256][]byte, reads []hash.Hash256, mutations map[hash.Hash256][]byte) (*ContractStorageWitness, []byte) {
	t.Helper()
	require := require.New(t)

	inner := newTestTrieWithAddr(t, _testAddr)
	for k, v := range prestateKVs {
		require.NoError(inner.Upsert(k[:], v))
	}

	wt := newWitnessTrie(inner, _testHashFuncAddr())

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
	postRoot, err := inner.RootHash()
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

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil)
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

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil)
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

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil)
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
	wt := newWitnessTrie(inner, _testHashFuncAddr())
	_, err := wt.Get(_k1b[:]) // record first touch (absent)
	require.Error(err)
	require.NoError(wt.Upsert(_k1b[:], _v1b[:]))

	postRoot, err := inner.RootHash()
	require.NoError(err)

	w, err := wt.BuildWitness()
	require.NoError(err)
	require.Equal(hash.ZeroHash256, w.StorageRoot)

	// Validator: replay on stateless MPT
	tr, err := newStatelessTrie(_testAddrHash160(), w, nil)
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

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil)
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
		_k2b: nil,      // delete k2
		_k3b: _v4b[:], // insert k3
	}
	reads := []hash.Hash256{_k1b, _k2b, _k3b}
	w, producerPostRoot := buildWitness(t, prestate, reads, mutations)

	tr, err := newStatelessTrie(_testAddrHash160(), w, nil)
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
	}, nil)
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
