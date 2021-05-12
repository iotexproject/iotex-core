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

	require.True(cfg.IsPrePacific(uint64(432000)))
	require.True(cfg.IsPostPacific(uint64(432001)))
	require.True(cfg.IsPreAleutian(uint64(864000)))
	require.True(cfg.IsPostAleutian(uint64(864001)))
	require.True(cfg.IsPreBering(uint64(1512000)))
	require.True(cfg.IsPostBering(uint64(1512001)))
	require.True(cfg.IsPreCook(uint64(1641600)))
	require.True(cfg.IsPostCook(uint64(1641601)))
	require.True(cfg.IsPreDardanelles(uint64(1816200)))
	require.True(cfg.IsPostDardanelles(uint64(1816201)))
	require.True(cfg.IsPreDaytona(uint64(3238920)))
	require.True(cfg.IsPostDaytona(uint64(3238921)))
	require.True(cfg.IsPreEaster(uint64(4478760)))
	require.True(cfg.IsPostEaster(uint64(4478761)))
	require.True(cfg.IsPreFairbank(uint64(5165640)))
	require.True(cfg.IsPostFairbank(uint64(5165641)))
	require.True(cfg.IsPreFbkMigration(uint64(5157000)))
	require.True(cfg.IsPostFbkMigration(uint64(5157001)))
	require.True(cfg.IsPreGreenland(uint64(6544440)))
	require.True(cfg.IsPostGreenland(uint64(6544441)))
	require.True(cfg.IsPreHawaii(uint64(11267640)))
	require.True(cfg.IsPostHawaii(uint64(11267641)))

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
}
