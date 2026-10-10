// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package erigonstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type namedObjectStorage struct {
	ObjectStorage
	name string
}

func TestKeySplitStorageMatchLongestPrefix(t *testing.T) {
	r := require.New(t)
	var (
		fallback = &namedObjectStorage{name: "fallback"}
		short    = &namedObjectStorage{name: "short"}
		long     = &namedObjectStorage{name: "long"}
	)
	rhs := newKeySplitContractStorageWithfallback(nil, nil, fallback, map[string]ObjectStorage{
		"a":   short,
		"ab":  long,
		"abd": &namedObjectStorage{name: "other"},
		"b":   &namedObjectStorage{name: "other"},
	})
	for range 100 {
		r.Equal(long, rhs.matchStorage([]byte("abc")))
		r.Equal(short, rhs.matchStorage([]byte("ac")))
		r.Equal(fallback, rhs.matchStorage([]byte("c")))
	}
}

func TestRegistryMatchLongestNamespacePrefix(t *testing.T) {
	r := require.New(t)
	osr := newObjectStorageRegistry()
	r.NoError(osr.RegisterNamespacePrefix("cs_", 1))
	r.NoError(osr.RegisterNamespacePrefix("cs_bucket_", 2))
	r.NoError(osr.RegisterNamespacePrefix("cs_bucket_type_", 3))
	r.NoError(osr.RegisterNamespacePrefix("other_", 4))
	for range 100 {
		index, ok := osr.matchContractIndex("cs_bucket_type_1", nil)
		r.True(ok)
		r.Equal(3, index)
		index, ok = osr.matchContractIndex("cs_bucket_1", nil)
		r.True(ok)
		r.Equal(2, index)
		index, ok = osr.matchContractIndex("cs_x", nil)
		r.True(ok)
		r.Equal(1, index)
		_, ok = osr.matchContractIndex("unknown", nil)
		r.False(ok)
	}
}
