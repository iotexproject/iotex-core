// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestNewServerV2(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, err := createServerV2(cfg, false)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()
	require.NoError(svr.Start())
	time.Sleep(5 * time.Second) //let server have enough time to start.
	require.NoError(svr.Stop())
}

func createServerV2(cfg config.Config, needActPool bool) (*ServerV2, string, error) {
	bc, dao, indexer, bfIndexer, sf, ap, registry, bfIndexFile, err := setupChain(cfg)
	if err != nil {
		return nil, "", err
	}

	ctx := context.Background()

	// Start blockchain
	if err := bc.Start(ctx); err != nil {
		return nil, "", err
	}
	// Add testing blocks
	if err := addTestingBlocks(bc, ap); err != nil {
		return nil, "", err
	}

	if needActPool {
		// Add actions to actpool
		ctx = protocol.WithRegistry(ctx, registry)
		if err := addActsToActPool(ctx, ap); err != nil {
			return nil, "", err
		}
	}
	svr, err := NewServerV2(cfg, bc, nil, sf, dao, indexer, bfIndexer, ap, registry,
		WithNativeElection(nil),
		WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
			return nil
		}))
	if err != nil {
		return nil, "", err
	}
	svr.core.hasActionIndex = true
	return svr, bfIndexFile, nil
}
