// Copyright (c) 2020 IoTeX
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
	require.Equal(5, Daytona)
	require.Equal(6, Easter)
	require.Equal(7, Fairbank)
	require.Equal(8, FbkMigration)
	require.Equal(9, Greenland)
	require.Equal(10, Hawaii)

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
	require.True(hu.IsPre(Daytona, uint64(3238920)))
	require.True(hu.IsPost(Daytona, uint64(3238921)))
	require.True(hu.IsPre(Easter, uint64(4478760)))
	require.True(hu.IsPost(Easter, uint64(4478761)))
	require.True(hu.IsPre(Fairbank, uint64(5165640)))
	require.True(hu.IsPost(Fairbank, uint64(5165641)))
	require.True(hu.IsPre(FbkMigration, uint64(5157000)))
	require.True(hu.IsPost(FbkMigration, uint64(5157001)))
	require.True(hu.IsPre(Greenland, uint64(6544440)))
	require.True(hu.IsPost(Greenland, uint64(6544441)))
	require.True(hu.IsPre(Hawaii, uint64(11073240)))
	require.True(hu.IsPost(Hawaii, uint64(11073241)))
	require.Panics(func() {
		hu.IsPost(-1, 0)
	})

	require.Equal(hu.PacificBlockHeight(), uint64(432001))
	require.Equal(hu.AleutianBlockHeight(), uint64(864001))
	require.Equal(hu.BeringBlockHeight(), uint64(1512001))
	require.Equal(hu.CookBlockHeight(), uint64(1641601))
	require.Equal(hu.DardanellesBlockHeight(), uint64(1816201))
	require.Equal(hu.DaytonaBlockHeight(), uint64(3238921))
	require.Equal(hu.EasterBlockHeight(), uint64(4478761))
	require.Equal(hu.FairbankBlockHeight(), uint64(5165641))
	require.Equal(hu.FbkMigrationBlockHeight(), uint64(5157001))
	require.Equal(hu.GreenlandBlockHeight(), uint64(6544441))
	require.Equal(hu.HawaiiBlockHeight(), uint64(11073241))
}
