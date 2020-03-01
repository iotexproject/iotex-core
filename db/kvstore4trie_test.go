// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/require"
)

func TestKVStoreForTrie_ErrNotExist(t *testing.T) {
	store, err := NewKVStoreForTrie("test", NewMemKVStore())
	require.NoError(t, err)
	require.NoError(t, store.Put([]byte("key"), []byte("value1")))
	require.NoError(t, store.Delete([]byte("key")))
	_, err = store.Get([]byte("key"))
	require.Equal(t, ErrNotExist, errors.Cause(err))
	// A non-existing key should return ErrNotExist too
	_, err = store.Get([]byte("key1"))
	require.Equal(t, ErrNotExist, errors.Cause(err))
}
