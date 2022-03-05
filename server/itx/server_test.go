// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestStop(t *testing.T) {
	require := require.New(t)
	cfg, cleanupPath := newConfig(t)
	defer cleanupPath()
	svr, err := NewServer(cfg)
	require.NoError(err)
	ctx := context.Background()
	err = svr.Start(ctx)
	require.NoError(err)
	err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
		err = svr.Stop(ctx)
		return err == nil, err
	})
	require.NoError(err)
}

func TestNewSubChainService(t *testing.T) {
	require := require.New(t)
	cfg, cleanupPath := newConfig(t)
	defer cleanupPath()
	svr, err := NewServer(cfg)
	require.NoError(err)
	err = svr.NewSubChainService(cfg)
	require.NoError(err)
	cs := svr.ChainService(1)
	require.NotNil(cs)
	err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
		err = svr.StopChainService(context.Background(), 1)
		return err == nil, err
	})
	require.NoError(err)
}

func TestStartServer(t *testing.T) {
	require := require.New(t)
	cfg, cleanupPath := newConfig(t)
	defer cleanupPath()
	svr, err := NewServer(cfg)
	require.NoError(err)
	probeSvr := probe.New(cfg.System.HTTPStatsPort)
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(probeSvr.Start(ctx))
	go func() {
		testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
			cancel()
			return true, nil
		})
	}()
	StartServer(ctx, svr, probeSvr, cfg)
}

func newConfig(t *testing.T) (config.Config, func()) {
	require := require.New(t)
	dbPath, err := testutil.PathOfTempFile("chain.db")
	require.NoError(err)
	triePath, err := testutil.PathOfTempFile("trie.db")
	require.NoError(err)
	cfg := config.Default
	cfg.API.Port = testutil.RandomPort()
	cfg.API.Web3Port = testutil.RandomPort()
	cfg.Chain.ChainDBPath = dbPath
	cfg.Chain.TrieDBPath = triePath
	cfg.Chain.TrieDBPatchFile = ""
	return cfg, func() {
		testutil.CleanupPathV2(dbPath)
		testutil.CleanupPathV2(triePath)
	}
}
