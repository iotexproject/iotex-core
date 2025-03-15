// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/probe"
	"github.com/iotexproject/iotex-core/v2/testutil"
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
	ctx := context.Background()
	require.NoError(cs.Start(ctx))
	err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
		err = svr.StopChainService(ctx, 1)
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
	indexPath, err := testutil.PathOfTempFile("indxer.db")
	require.NoError(err)
	blobPath, err := testutil.PathOfTempFile("blob.db")
	require.NoError(err)
	contractIndexPath, err := testutil.PathOfTempFile("contractindxer.db")
	require.NoError(err)
	testActionStorePath := t.TempDir()
	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = testutil.RandomPort()
	cfg.Chain.ChainDBPath = dbPath
	cfg.Chain.TrieDBPath = triePath
	cfg.Chain.BlobStoreDBPath = blobPath
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.BlobStoreDBPath = blobPath
	cfg.Chain.ContractStakingIndexDBPath = contractIndexPath
	if cfg.ActPool.Store != nil {
		cfg.ActPool.Store.Datadir = testActionStorePath
	}
	return cfg, func() {
		testutil.CleanupPath(dbPath)
		testutil.CleanupPath(triePath)
		testutil.CleanupPath(indexPath)
		testutil.CleanupPath(blobPath)
		testutil.CleanupPath(contractIndexPath)
		testutil.CleanupPath(blobPath)
		testutil.CleanupPath(testActionStorePath)
	}
}
