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
	require.Equal(2, Bering)
	require.Equal(3, Cook)
	require.Equal(4, Dardanelles)
	require.Equal(5, English)

	cfg := Default
	cfg.Genesis.PacificBlockHeight = uint64(432001)
	hu := NewHeightUpgrade(&cfg.Genesis)

	require.True(hu.IsPre(Pacific, uint64(432000)))
	require.True(hu.IsPost(Pacific, uint64(432001)))
	require.True(hu.IsPre(Aleutian, uint64(864000)))
	require.True(hu.IsPost(Aleutian, uint64(864001)))
	require.True(hu.IsPre(Bering, uint64(1512000)))
	require.True(hu.IsPost(Bering, uint64(1512001)))
	require.True(hu.IsPre(Cook, uint64(1641600)))
	require.True(hu.IsPost(Cook, uint64(1641601)))
	require.True(hu.IsPre(Dardanelles, uint64(1816200)))
	require.True(hu.IsPost(Dardanelles, uint64(1816201)))
	require.True(hu.IsPre(English, uint64(3363480)))
	require.True(hu.IsPost(English, uint64(3363481)))
	require.Panics(func() {
		hu.IsPost(-1, 0)
	})

	require.Equal(hu.PacificBlockHeight(), uint64(432001))
	require.Equal(hu.AleutianBlockHeight(), uint64(864001))
	require.Equal(hu.BeringBlockHeight(), uint64(1512001))
	require.Equal(hu.CookBlockHeight(), uint64(1641601))
	require.Equal(hu.DardanellesBlockHeight(), uint64(1816201))
	require.Equal(hu.EnglishBlockHeight(), uint64(3363481))

}
