// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package genesis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHeightChange(t *testing.T) {
	require := require.New(t)

	cfg := Default
	cfg.PacificBlockHeight = uint64(432001)

	require.False(cfg.IsPacific(uint64(432000)))
	require.True(cfg.IsPacific(uint64(432001)))
	require.False(cfg.IsAleutian(uint64(864000)))
	require.True(cfg.IsAleutian(uint64(864001)))
	require.False(cfg.IsBering(uint64(1512000)))
	require.True(cfg.IsBering(uint64(1512001)))
	require.False(cfg.IsCook(uint64(1641600)))
	require.True(cfg.IsCook(uint64(1641601)))
	require.False(cfg.IsDardanelles(uint64(1816200)))
	require.True(cfg.IsDardanelles(uint64(1816201)))
	require.False(cfg.IsDaytona(uint64(3238920)))
	require.True(cfg.IsDaytona(uint64(3238921)))
	require.False(cfg.IsEaster(uint64(4478760)))
	require.True(cfg.IsEaster(uint64(4478761)))
	require.False(cfg.IsFairbank(uint64(5165640)))
	require.True(cfg.IsFairbank(uint64(5165641)))
	require.False(cfg.IsFbkMigration(uint64(5157000)))
	require.True(cfg.IsFbkMigration(uint64(5157001)))
	require.False(cfg.IsGreenland(uint64(6544440)))
	require.True(cfg.IsGreenland(uint64(6544441)))
	require.False(cfg.IsHawaii(uint64(11267640)))
	require.True(cfg.IsHawaii(uint64(11267641)))
	require.False(cfg.IsIceland(uint64(12289320)))
	require.True(cfg.IsIceland(uint64(12289321)))
	require.False(cfg.IsJutland(uint64(13685400)))
	require.True(cfg.IsJutland(uint64(13685401)))
	require.False(cfg.IsKamchatka(uint64(13816440)))
	require.True(cfg.IsKamchatka(uint64(13816441)))
	require.False(cfg.IsLordHowe(uint64(13979160)))
	require.True(cfg.IsLordHowe(uint64(13979161)))
	require.False(cfg.IsMidway(uint64(33816440)))
	require.True(cfg.IsMidway(uint64(33816441)))

	require.Equal(cfg.PacificBlockHeight, uint64(432001))
	require.Equal(cfg.AleutianBlockHeight, uint64(864001))
	require.Equal(cfg.BeringBlockHeight, uint64(1512001))
	require.Equal(cfg.CookBlockHeight, uint64(1641601))
	require.Equal(cfg.DardanellesBlockHeight, uint64(1816201))
	require.Equal(cfg.DaytonaBlockHeight, uint64(3238921))
	require.Equal(cfg.EasterBlockHeight, uint64(4478761))
	require.Equal(cfg.FairbankBlockHeight, uint64(5165641))
	require.Equal(cfg.FbkMigrationBlockHeight, uint64(5157001))
	require.Equal(cfg.GreenlandBlockHeight, uint64(6544441))
	require.Equal(cfg.HawaiiBlockHeight, uint64(11267641))
	require.Equal(cfg.IcelandBlockHeight, uint64(12289321))
	require.Equal(cfg.JutlandBlockHeight, uint64(13685401))
	require.Equal(cfg.KamchatkaBlockHeight, uint64(13816441))
	require.Equal(cfg.LordHoweBlockHeight, uint64(13979161))
	require.Equal(cfg.MidwayBlockHeight, uint64(33816441))
}
