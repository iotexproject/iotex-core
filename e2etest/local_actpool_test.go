// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestLocalActPool(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	cfg, err := newActPoolConfig()
	require.NoError(err)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)

	chainID := cfg.Chain.ID
	require.NoError(svr.Start(ctx))

	require.NotNil(svr.ChainService(chainID).ActionPool())

	// create client
	cfg, err = newActPoolConfig()
	require.NoError(err)
	cfg.Network.BootstrapNodes = []string{svr.P2PAgent().Self()[0].String()}
	cli := p2p.NewAgent(
		cfg.Network,
		func(_ context.Context, _ uint32, _ proto.Message) {

		},
		func(_ context.Context, _ uint32, _ peerstore.PeerInfo, _ proto.Message) {

		},
	)
	require.NotNil(cli)
	require.NoError(cli.Start(ctx))

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	// Create three valid actions from "from" to "to"
	tsf1, err := testutil.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), 1, big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	p2pCtx := p2p.WitContext(ctx, p2p.Context{ChainID: chainID})
	// Wait until server receives the 1st action
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		require.NoError(cli.BroadcastOutbound(p2pCtx, tsf1.Proto()))
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	}))

	vote2, err := testutil.SignedVote(identityset.Address(1).String(), identityset.PrivateKey(1), 2, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), 3, big.NewInt(3), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Create contract
	exec4, err := testutil.SignedExecution(action.EmptyAddress, identityset.PrivateKey(1), 4, big.NewInt(0), uint64(120000), big.NewInt(10), []byte{})
	require.NoError(err)
	// Create three invalid actions from "from" to "to"
	// Existed Vote
	vote5, err := testutil.SignedVote(identityset.Address(0).String(), identityset.PrivateKey(1), 2, uint64(100000), big.NewInt(0))
	require.NoError(err)

	require.NoError(cli.BroadcastOutbound(p2pCtx, vote2.Proto()))
	require.NoError(cli.BroadcastOutbound(p2pCtx, tsf3.Proto()))
	require.NoError(cli.BroadcastOutbound(p2pCtx, exec4.Proto()))
	require.NoError(cli.BroadcastOutbound(p2pCtx, vote5.Proto()))

	// Wait until server receives all the transfers
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		// 2 valid transfers and 1 valid vote and 1 valid execution
		return len(acts) == 4, nil
	}))
}

func TestPressureActPool(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	cfg, err := newActPoolConfig()
	require.NoError(err)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)
	require.Nil(svr.Start(ctx))
	chainID := cfg.Chain.ID
	require.NotNil(svr.ChainService(chainID).ActionPool())

	// create client
	cfg, err = newActPoolConfig()
	require.NoError(err)
	cfg.Network.BootstrapNodes = []string{svr.P2PAgent().Self()[0].String()}
	cli := p2p.NewAgent(
		cfg.Network,
		func(_ context.Context, _ uint32, _ proto.Message) {

		},
		func(_ context.Context, _ uint32, _ peerstore.PeerInfo, _ proto.Message) {

		},
	)
	require.NotNil(cli)
	require.Nil(cli.Start(ctx))

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	p2pCtx := p2p.WitContext(ctx, p2p.Context{ChainID: chainID})
	tsf, err := testutil.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), 1, big.NewInt(int64(0)), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Wait until server receives the 1st action
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		require.NoError(cli.BroadcastOutbound(p2pCtx, tsf.Proto()))
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	}))

	for i := 2; i <= 1000; i++ {
		tsf, err := testutil.SignedTransfer(identityset.Address(0).String(), identityset.PrivateKey(1), uint64(i), big.NewInt(int64(i)), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		require.NoError(cli.BroadcastOutbound(p2pCtx, tsf.Proto()))
	}

	// Wait until committed blocks contain all broadcasted actions
	err = testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1000, nil
	})
	require.Nil(err)
}

func newActPoolConfig() (config.Config, error) {
	cfg := config.Default
	cfg.NodeType = config.DelegateType
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.EnableIndex = true
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()
	cfg.Explorer.Enabled = true
	cfg.Explorer.Port = 0

	sk, err := crypto.GenerateKey()
	if err != nil {
		return config.Config{}, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(&sk.PublicKey)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(sk)
	return cfg, nil
}
