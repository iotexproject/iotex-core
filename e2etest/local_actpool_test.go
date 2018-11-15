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

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	// Make sure the key pairs used here match the genesis block
	// Sender's public/private key pair
	fromPubKey  = "d857218c61a15a71f9ac313d954b12f4a0748d88840eb3b6b2cffa727e86f73fe57ade02d65e80df2078b5de771e552a7b264450b3c9daebb42e96c80c5e6e708e8f1105f11bdc03"
	fromPrivKey = "2a67356f088788959d4fb2754d15e8bd4b30252dede329609c8c604504fda1a7c87d5d00"
	// Recipient's public/private key pair
	toPubKey = "3b09b24c31c27161e35b6dc897a95a1df16fa7d7e2f73ce0d15babe58cb17e6dfab87b000c199e5a6113f5da3fc621ce1145650e921781d57c5b65e8ecefe0238419a8c843557b00"
)

func TestLocalActPool(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	cfg, err := newActPoolConfig()
	require.NoError(err)

	blockchain.Gen.BlockReward = big.NewInt(0)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)

	chainID := cfg.Chain.ID
	require.NoError(svr.Start(ctx))

	require.NotNil(svr.ChainService(chainID).ActionPool())

	// create client
	cfg.Network.BootstrapNodes = []string{svr.P2P().Self().String()}
	cli := network.NewOverlay(&cfg.Network)
	require.NotNil(cli)
	require.NoError(cli.Start(ctx))

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	from := testaddress.ConstructAddress(chainID, fromPubKey, fromPrivKey)
	to := testaddress.ConstructAddress(chainID, toPubKey, "")

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		return len(svr.P2P().GetPeers()) == 1 && len(cli.GetPeers()) == 1, nil
	}))

	// Create three valid actions from "from" to "to"
	tsf1, err := testutil.SignedTransfer(from, to, uint64(1), big.NewInt(1),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	vote2, err := testutil.SignedVote(from, from, uint64(2), uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(from, to, uint64(3), big.NewInt(3),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Create contract
	exec4, err := testutil.SignedExecution(from, action.EmptyAddress, uint64(4), big.NewInt(0),
		uint64(120000), big.NewInt(10), []byte{})
	require.NoError(err)
	// Create three invalid actions from "from" to "to"
	// Existed Vote
	vote5, err := testutil.SignedVote(from, from, uint64(2), uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Unsigned Vote
	vote6, err := action.NewVote(uint64(7), from.RawAddress, from.RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(err)

	require.NoError(cli.Broadcast(chainID, tsf1.Proto()))
	require.NoError(cli.Broadcast(chainID, vote2.Proto()))
	require.NoError(cli.Broadcast(chainID, tsf3.Proto()))
	require.NoError(cli.Broadcast(chainID, exec4.Proto()))
	require.NoError(cli.Broadcast(chainID, vote5.Proto()))
	require.NoError(cli.Broadcast(chainID, vote6.Proto()))

	// Wait until server receives all the transfers
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
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

	blockchain.Gen.BlockReward = big.NewInt(0)

	// create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.Nil(err)
	require.Nil(svr.Start(ctx))
	chainID := cfg.Chain.ID
	require.NotNil(svr.ChainService(chainID).ActionPool())

	// create client
	cfg.Network.BootstrapNodes = []string{svr.P2P().Self().String()}
	cli := network.NewOverlay(&cfg.Network)
	require.NotNil(cli)
	require.Nil(cli.Start(ctx))

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	from := testaddress.ConstructAddress(chainID, fromPubKey, fromPrivKey)
	to := testaddress.ConstructAddress(chainID, toPubKey, "")

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		return len(svr.P2P().GetPeers()) == 1 && len(cli.GetPeers()) == 1, nil
	}))

	require.Nil(err)
	for i := 1; i <= 1000; i++ {
		tsf, err := testutil.SignedTransfer(from, to, uint64(i), big.NewInt(int64(i)),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		require.NoError(cli.Broadcast(chainID, tsf.Proto()))
	}

	// Wait until committed blocks contain all broadcasted actions
	err = testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1000, nil
	})
	require.Nil(err)
}

func newActPoolConfig() (*config.Config, error) {
	cfg := config.Default
	cfg.NodeType = config.DelegateType
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.InMemTest = false
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = 0
	cfg.Network.PeerMaintainerInterval = 100 * time.Millisecond
	cfg.Explorer.Enabled = true
	cfg.Explorer.Port = 0

	pk, sk, err := crypto.EC283.NewKeyPair()
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(pk)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(sk)
	return &cfg, nil
}
