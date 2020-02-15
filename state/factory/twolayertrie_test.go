// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
)

func TestTwoLayerTrie(t *testing.T) {
	kvStoreForTrie, err := db.NewKVStoreForTrie("default", db.NewMemKVStore())
	require.NoError(t, err)
	tr, err := trie.NewTrie(trie.KVStoreOption(kvStoreForTrie))
	require.NoError(t, err)
	tlt := &TwoLayerTrie{layerOne: tr}
	require.NoError(t, tlt.Start(context.Background()))
	defer require.NoError(t, tlt.Stop(context.Background()))
	require.Equal(t, tr.RootHash(), tlt.RootHash())
	_, err = tlt.Get([]byte("layerOneKey111111111"), []byte("layerTwoKey1"))
	require.Error(t, err)
	require.Error(t, tlt.Delete([]byte("layerOneKey111111111"), []byte("layerTwoKey1")))
	require.NoError(t, tlt.Upsert([]byte("layerOneKey111111111"), []byte("layerTwoKey1"), []byte("value")))
	_, err = tlt.Get([]byte("layerOneKey111111111"), []byte("layerTwoKey2"))
	require.Error(t, err)
	value, err := tlt.Get([]byte("layerOneKey111111111"), []byte("layerTwoKey1"))
	require.NoError(t, err)
	require.Equal(t, []byte("value"), value)
	require.Error(t, tlt.Delete([]byte("layerOneKey111111111"), []byte("layerTwoKey2")))
	require.NoError(t, tlt.Delete([]byte("layerOneKey111111111"), []byte("layerTwoKey1")))
	require.True(t, tr.IsEmpty())
}
