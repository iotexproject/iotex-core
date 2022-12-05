// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db/trie"
)

func TestIterator(t *testing.T) {
	var (
		require = require.New(t)
		items   = []struct{ k, v string }{
			{"iotex", "coin"},
			{"block", "chain"},
			{"chain", "link"},
			{"puppy", "dog"},
			{"night", "knight"},
		}
		memStore = trie.NewMemKVStore()
	)

	mpt, err := New(KVStoreOption(memStore), KeyLengthOption(5), AsyncOption())
	require.NoError(err)
	require.NoError(mpt.Start(context.Background()))

	for _, item := range items {
		require.NoError(mpt.Upsert([]byte(item.k), []byte(item.v)))
	}

	iter, err := NewLeafIterator(mpt)
	require.NoError(err)

	found := make(map[string]string)
	for {
		k, v, err := iter.Next()
		if err != nil {
			require.Equal(trie.ErrEndOfIterator, err)
			break
		}
		found[string(k)] = string(v)
	}

	for _, item := range items {
		require.Equal(item.v, found[item.k], "key: %s", item.k)
	}
}
