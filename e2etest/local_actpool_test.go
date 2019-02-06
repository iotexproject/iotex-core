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
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	// Make sure the key pairs used here match the genesis block
	// Sender's public/private key pair
	fromPubKey  = "046fd639e777d7ebc441a2f594994f99b3cce0e7c9bc47aa2c71b50b14c561efbc8762b915608fc97b79ba81aa555ef2b5b275c5cc0405873039d67749ffce3476"
	fromPrivKey = "bb8f0b63179590f1f47c9613b00819d8869591ccdf654c363f5825ac3d05d45f"
	// Recipient's public key
	toPubKey = "0415efbeb0cabe6e920afdff0558e34be9f17f3ef3a70bb60d5eb6c00f517c2f870da979eed4e60deeb627d6335544aa514b25f0225063f4094cedc0ef0b49a316"
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

	fromPK, _ := keypair.DecodePublicKey(fromPubKey)
	toPK, _ := keypair.DecodePublicKey(toPubKey)

	fromPKHash := keypair.HashPubKey(fromPK)
	toPKHash := keypair.HashPubKey(toPK)

	from := address.New(fromPKHash[:]).Bech32()
	to := address.New(toPKHash[:]).Bech32()
	priKey, _ := keypair.DecodePrivateKey(fromPrivKey)

	// Create three valid actions from "from" to "to"
	tsf1, err := testutil.SignedTransfer(from, to, priKey, uint64(1), big.NewInt(1),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	p2pCtx := p2p.WitContext(ctx, p2p.Context{ChainID: chainID})
	// Wait until server receives the 1st action
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		require.NoError(cli.BroadcastOutbound(p2pCtx, tsf1.Proto()))
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	}))

	vote2, err := testutil.SignedVote(from, from, priKey, uint64(2), uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf3, err := testutil.SignedTransfer(from, to, priKey, uint64(3), big.NewInt(3),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Create contract
	exec4, err := testutil.SignedExecution(from, action.EmptyAddress, priKey, uint64(4), big.NewInt(0),
		uint64(120000), big.NewInt(10), []byte{})
	require.NoError(err)
	// Create three invalid actions from "from" to "to"
	// Existed Vote
	vote5, err := testutil.SignedVote(from, from, priKey, uint64(2), uint64(100000), big.NewInt(0))
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

	fromPK, _ := keypair.DecodePublicKey(fromPubKey)
	toPK, _ := keypair.DecodePublicKey(toPubKey)

	fromPKHash := keypair.HashPubKey(fromPK)
	toPKHash := keypair.HashPubKey(toPK)

	from := address.New(fromPKHash[:]).Bech32()
	to := address.New(toPKHash[:]).Bech32()
	priKey, _ := keypair.DecodePrivateKey(fromPrivKey)

	p2pCtx := p2p.WitContext(ctx, p2p.Context{ChainID: chainID})
	tsf, err := testutil.SignedTransfer(from, to, priKey, 1, big.NewInt(int64(0)),
		[]byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	// Wait until server receives the 1st action
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		require.NoError(cli.BroadcastOutbound(p2pCtx, tsf.Proto()))
		acts := svr.ChainService(chainID).ActionPool().PickActs()
		return len(acts) == 1, nil
	}))

	for i := 2; i <= 1000; i++ {
		tsf, err := testutil.SignedTransfer(from, to, priKey, uint64(i), big.NewInt(int64(i)),
			[]byte{}, uint64(100000), big.NewInt(0))
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
