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

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	// Make sure the key pairs used here match the genesis block
	// Sender's public/private key pair
	fromPubKey  = "2726440bc26449be22eb5c0564af4b23dc8c373aa79e8cb0f8df2a9e55b4842dbefcde07d95c1dc1f3d1a367086b4f7742115b53c434e8f5abf116333c2c378c51b0ef6176153602"
	fromPrivKey = "c5364b1a2d99d127439be22edfd657889981e9ba4d6d18fe8eca489d48485371efcb2400"
	// Recipient's public/private key pair
	toPubKey  = "2ba2e72613783656b92af930719d2a13874bcb4999b7a0ae11a5beb469357da441f41303dc1ad5a4e6c0cdde85ceb11516bbcaca68bb82168255de60e3a216f00c18c1285a3d4402"
	toPrivKey = "ededa5274f44b4075dcabfe4d1aaa4f7f50ca21bc1f4c5a676dd11727d21b344c575a401"
)

func TestLocalActPool(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	cfg, err := newActPoolConfig()
	require.NoError(err)

	blockchain.Gen.BlockReward = uint64(0)

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

	from := testutil.ConstructAddress(chainID, fromPubKey, fromPrivKey)
	to := testutil.ConstructAddress(chainID, toPubKey, toPrivKey)

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

	require.NoError(cli.Broadcast(chainID, tsf1.ConvertToActionPb()))
	require.NoError(cli.Broadcast(chainID, vote2.ConvertToActionPb()))
	require.NoError(cli.Broadcast(chainID, tsf3.ConvertToActionPb()))
	require.NoError(cli.Broadcast(chainID, exec4.ConvertToActionPb()))
	require.NoError(cli.Broadcast(chainID, vote5.ConvertToActionPb()))
	require.NoError(cli.Broadcast(chainID, vote6.ConvertToActionPb()))

	// Wait until server receives all the transfers
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		transfers, votes, executions, _ := svr.ChainService(chainID).ActionPool().PickActs()
		// 2 valid transfers and 1 valid vote and 1 valid execution
		return len(transfers) == 2 && len(votes) == 1 && len(executions) == 1, nil
	}))
}

func TestPressureActPool(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	cfg, err := newActPoolConfig()
	require.NoError(err)

	blockchain.Gen.BlockReward = uint64(0)

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

	from := testutil.ConstructAddress(chainID, fromPubKey, fromPrivKey)
	to := testutil.ConstructAddress(chainID, toPubKey, toPrivKey)

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		return len(svr.P2P().GetPeers()) == 1 && len(cli.GetPeers()) == 1, nil
	}))

	require.Nil(err)
	for i := 1; i <= 1000; i++ {
		tsf, err := testutil.SignedTransfer(from, to, uint64(i), big.NewInt(int64(i)),
			[]byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)
		require.NoError(cli.Broadcast(chainID, tsf.ConvertToActionPb()))
	}

	// Wait until committed blocks contain all broadcasted actions
	err = testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		transfers, _, _, _ := svr.ChainService(chainID).ActionPool().PickActs()
		return len(transfers) == 1000, nil
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
	cfg.Explorer.Port = 0

	pk, sk, err := crypto.EC283.NewKeyPair()
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(pk)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(sk)
	return &cfg, nil
}
