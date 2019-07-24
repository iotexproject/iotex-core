// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHeightChange(t *testing.T) {
	require := require.New(t)

	require.Equal(0, Pacific)
	require.Equal(1, Aleutian)
	cfg := Default
	cfg.Genesis.PacificBlockHeight = uint64(432001)
	hu := NewHeightUpgrade(cfg)
	require.Equal(uint64(432001), hu.pacificHeight)
	require.Equal(uint64(864001), hu.aleutianHeight)
	require.Equal(uint64(1106641), hu.beringHeight)

	require.True(hu.IsPre(Pacific, uint64(432000)))
	require.True(hu.IsPost(Pacific, uint64(432001)))
	require.True(hu.IsPre(Aleutian, uint64(864000)))
	require.True(hu.IsPost(Aleutian, uint64(864001)))
	require.True(hu.IsPre(Bering, uint64(1106640)))
	require.True(hu.IsPost(Bering, uint64(1106641)))
}
