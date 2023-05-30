// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestLocalActPool(t *testing.T) {
	require := require.New(t)

	cfg, err := newActPoolConfig(t)
	require.NoError(err)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.NoError(err)

	chainID := cfg.Chain.ID
	fmt.Println("server start")
	require.NoError(svr.Start(ctx))
	fmt.Println("server started")
	require.NotNil(svr.ChainService(chainID).ActionPool())

	// create client
	cfg, err = newActPoolConfig(t)
	require.NoError(err)
	addrs, err := svr.P2PAgent().Self()
	require.NoError(err)
	cfg.Network.BootstrapNodes = []string{validNetworkAddr(addrs)}
	cli := p2p.NewAgent(
		cfg.Network,
		cfg.Chain.ID,
		cfg.Genesis.Hash(),
		func(_ context.Context, _ uint32, _ string, _ proto.Message) {

		},
		func(_ context.Context, _ uint32, _ peer.AddrInfo, _ proto.Message) {

		},
	)
	require.NotNil(cli)
	require.NoError(cli.Start(ctx))
	fmt.Println("p2p agent started")

	defer func() {
		require.NoError(cli.Stop(ctx))
		require.NoError(svr.Stop(ctx))
	}()

	// Create three valid actions from "from" to "to"
	tsf1, err := action.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), 1, big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Wait until server receives the 1st action
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		require.NoError(cli.BroadcastOutbound(ctx, tsf1.Proto()))
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 1, nil
	}))
	fmt.Println("1")

	tsf2, err := action.SignedTransfer(identityset.Address(1).String(), identityset.PrivateKey(1), 2, big.NewInt(3), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := action.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), 3, big.NewInt(3), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Create contract
	exec4, err := action.SignedExecution(action.EmptyAddress, identityset.PrivateKey(1), 4, big.NewInt(0), uint64(120000), big.NewInt(10), []byte{})
	require.NoError(err)
	// Create three invalid actions from "from" to "to"
	tsf5, err := action.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), 2, big.NewInt(3), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)

	require.NoError(cli.BroadcastOutbound(ctx, tsf2.Proto()))
	require.NoError(cli.BroadcastOutbound(ctx, tsf3.Proto()))
	require.NoError(cli.BroadcastOutbound(ctx, exec4.Proto()))
	require.NoError(cli.BroadcastOutbound(ctx, tsf5.Proto()))

	fmt.Println("2")
	// Wait until server receives all the transfers
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		// 3 valid transfers and 1 valid execution
		return lenPendingActionMap(acts) == 4, nil
	}))
	fmt.Println("3")

}

func TestPressureActPool(t *testing.T) {
	require := require.New(t)

	cfg, err := newActPoolConfig(t)
	require.NoError(err)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))
	chainID := cfg.Chain.ID
	require.NotNil(svr.ChainService(chainID).ActionPool())

	// create client
	cfg, err = newActPoolConfig(t)
	require.NoError(err)
	addrs, err := svr.P2PAgent().Self()
	require.NoError(err)
	cfg.Network.BootstrapNodes = []string{validNetworkAddr(addrs)}
	cli := p2p.NewAgent(
		cfg.Network,
		cfg.Chain.ID,
		cfg.Genesis.Hash(),
		func(_ context.Context, _ uint32, _ string, _ proto.Message) {

		},
		func(_ context.Context, _ uint32, _ peer.AddrInfo, _ proto.Message) {

		},
	)
	require.NotNil(cli)
	require.NoError(cli.Start(ctx))

	defer func() {
		require.NoError(cli.Stop(ctx))
		require.NoError(svr.Stop(ctx))
	}()

	tsf, err := action.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), 1, big.NewInt(int64(0)), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Wait until server receives the 1st action
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		require.NoError(cli.BroadcastOutbound(ctx, tsf.Proto()))
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 1, nil
	}))

	// Broadcast has rate limit at 300
	for i := 2; i <= 250; i++ {
		tsf, err := action.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), uint64(i), big.NewInt(int64(i)), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		require.NoError(cli.BroadcastOutbound(ctx, tsf.Proto()))
	}

	// Wait until committed blocks contain all broadcasted actions
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		acts := svr.ChainService(chainID).ActionPool().PendingActionMap()
		return lenPendingActionMap(acts) == 250, nil
	})
	require.NoError(err)
}

func newActPoolConfig(t *testing.T) (config.Config, error) {
	r := require.New(t)

	cfg := config.Default

	testTriePath, err := testutil.PathOfTempFile("trie")
	r.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	r.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	r.NoError(err)
	testContractIndexPath, err := testutil.PathOfTempFile("contractindex")
	r.NoError(err)
	testSGDIndexPath, err := testutil.PathOfTempFile("sgdindex")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
		testutil.CleanupPath(testContractIndexPath)
		testutil.CleanupPath(testSGDIndexPath)
	}()

	cfg.Chain.TrieDBPatchFile = ""
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.Chain.ContractStakingIndexDBPath = testContractIndexPath
	cfg.Chain.SGDIndexDBPath = testSGDIndexPath
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()

	sk, err := crypto.GenerateKey()
	if err != nil {
		return config.Config{}, err
	}
	cfg.Chain.ProducerPrivKey = sk.HexString()
	return cfg, nil
}
