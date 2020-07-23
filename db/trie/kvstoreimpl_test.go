// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db"
)

func TestKVStore_Get(t *testing.T) {
	var (
		require = require.New(t)
		tests   = []struct{ k, v []byte }{
			{[]byte("key"), []byte("value")},
			{[]byte("k"), []byte("v")},
			{[]byte("iotex"), []byte("coin")},
			{[]byte("puppy"), []byte("dog")},
		}
	)

	store, err := NewKVStore("test", db.NewMemKVStore())
	require.NoError(err)

	for _, test := range tests {
		require.NoError(store.Put(test.k, test.v))
	}
	for _, test := range tests {
		val, err := store.Get(test.k)
		require.NoError(err)
		require.Equal(test.v, val)
	}
}

func TestKvStore_Put(t *testing.T) {
	var (
		require = require.New(t)
		key     = []byte("key")
		tests   = []struct{ k, v []byte }{
			{key, []byte("value1")},
			{key, []byte("value2")},
			{key, []byte("value3")},
		}
	)

	store, err := NewKVStore("test", db.NewMemKVStore())
	require.NoError(err)

	for _, test := range tests {
		require.NoError(store.Put(test.k, test.v))
	}
	val, err := store.Get(key)
	require.NoError(err)
	require.Equal(tests[len(tests)-1].v, val)
}

func TestKvStore_Delete(t *testing.T) {
	var (
		require = require.New(t)
		tests   = []struct{ k, v []byte }{
			{[]byte("key"), []byte("value")},
			{[]byte("k"), []byte("v")},
			{[]byte("iotex"), []byte("coin")},
			{[]byte("puppy"), []byte("dog")},
		}
	)

	store, err := NewKVStore("test", db.NewMemKVStore())
	require.NoError(err)

	for _, test := range tests {
		_, err = store.Get(test.k)
		require.Equal(ErrNotExist, errors.Cause(err))

		require.NoError(store.Put(test.k, test.v))
	}
	for _, test := range tests {
		val, err := store.Get(test.k)
		require.NoError(err)
		require.Equal(test.v, val)

		require.NoError(store.Delete(test.k))
		_, err = store.Get(test.k)
		require.Equal(ErrNotExist, errors.Cause(err))
	}
}
