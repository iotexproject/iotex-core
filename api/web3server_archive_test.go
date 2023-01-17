// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestWeb3ServerArchive(t *testing.T) {
	svr, _, _, _, cleanIndexFile := setupArchiveServer()
	web3svr := svr.httpSvr
	defer cleanIndexFile()
	ctx := context.Background()
	web3svr.Start(ctx)
	defer web3svr.Stop(ctx)
	handler := newHTTPHandler(NewWeb3Handler(svr.core, ""))

	// send request
	t.Run("eth_getBlockByNumber", func(t *testing.T) {
		getBlockByNumberArchive(t, handler)
	})
}

func createServerV2Archive(cfg testConfig, needActPool bool) (*ServerV2, blockchain.Blockchain, blockdao.BlockDAO, blockindex.Indexer, *protocol.Registry, actpool.ActPool, string, error) {
	// TODO (zhi): revise
	bc, dao, indexer, bfIndexer, sf, ap, registry, bfIndexFile, err := setupChain(cfg)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", err
	}

	ctx := context.Background()

	// Start blockchain
	if err := bc.Start(ctx); err != nil {
		return nil, nil, nil, nil, nil, nil, "", err
	}

	if needActPool {
		// Add actions to actpool
		ctx = protocol.WithRegistry(ctx, registry)
		if err := addActsToActPool(ctx, ap); err != nil {
			return nil, nil, nil, nil, nil, nil, "", err
		}
	}
	opts := []Option{WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
		return nil
	})}
	svr, err := NewServerV2(cfg.api, bc, nil, sf, dao, indexer, bfIndexer, ap, registry, opts...)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, "", err
	}
	return svr, bc, dao, indexer, registry, ap, bfIndexFile, nil
}

func setupArchiveServer() (*ServerV2, blockchain.Blockchain, blockdao.BlockDAO, actpool.ActPool, func()) {
	cfg := newConfig()
	cfg.chain.EVMNetworkID = _evmNetworkID
	cfg.api.HTTPPort = testutil.RandomPort()
	svr, bc, dao, _, _, actPool, bfIndexFile, _ := createServerV2Archive(cfg, false)
	return svr, bc, dao, actPool, func() {
		testutil.CleanupPath(bfIndexFile)
	}
}

func getBlockByNumberArchive(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{`["1", true]`, 1},
		{`["1", false]`, 2},
		{`["10", false]`, 0},
	} {
		result := serveTestHTTP(require, handler, "eth_getBlockByNumber", test.params)
		if test.expected == 0 {
			require.Nil(result)
		} else {
			actual, ok := result.(map[string]interface{})["transactions"]
			require.True(ok)
			require.Equal(test.expected, len(actual.([]interface{})))
		}
	}
}
