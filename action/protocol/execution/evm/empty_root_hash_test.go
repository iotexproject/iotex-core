package evm

import (
	"context"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
)

func TestEmptyTrieRootHash(t *testing.T) {
	require := require.New(t)

	// Create an empty MPT trie similar to how newContract creates it
	tr, err := mptrie.New(
		mptrie.KVStoreOption(trie.NewMemKVStore()),
		mptrie.KeyLengthOption(len(hash.Hash256{})),
		mptrie.HashFuncOption(func(data []byte) []byte {
			h := hash.Hash256b(data)
			return h[:]
		}),
	)
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))

	rh, err := tr.RootHash()
	require.NoError(err)

	emptyTrieRoot := hash.BytesToHash256(rh)

	// Key assertion: empty trie root hash is NOT zero hash
	require.NotEqual(hash.ZeroHash256, emptyTrieRoot,
		"empty trie root hash should NOT be zero hash")

	// SetRootHash(ZeroHash256) fails because the MPT trie doesn't recognize
	// zero hash as empty. This is the existing behavior that we must preserve
	// to avoid breaking historical block replay.
	require.Error(tr.SetRootHash(hash.ZeroHash256[:]),
		"SetRootHash(ZeroHash256) must fail to preserve historical block compatibility")
}
