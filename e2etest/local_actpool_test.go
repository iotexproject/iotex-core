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
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	pb "github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	// Make sure the key pairs used here match the genesis block
	// Sender's public/private key pair
	fromPubKey  = "408596b2dfe484c870c38146ca82ab0525a53978286dfd32866a757c080fd8cd4632b60649595ab5342fd560aabbe313cc2c02f0f6aff00e37965d6982abc84de4799f5e59884e06"
	fromPrivKey = "eb0281546a98eb84aec2cd083a5e41478818275d18aea7fb2d0755d73a4d5b3b37370a00"
	// Recipient's public/private key pair
	toPubKey  = "63c098f9fbeafc1353362710ef7ad30e355114bb1531d77c055d61bc555f86a9bcda6206e73309a2c32996624bb41eca7a8db33421dea1b30f67bb6c8e7a37302e10d27617abc905"
	toPrivKey = "277719b669a950cdd70872c87c7db0a4c4b24936650706ab750d5f0ea77c6301282ec800"
)

func TestLocalActPool(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	cfg, err := newActPoolConfig()
	require.NoError(err)

	blockchain.Gen.TotalSupply = uint64(50 << 22)
	blockchain.Gen.BlockReward = uint64(0)

	// create server
	ctx := context.Background()
	svr := itx.NewServer(cfg)
	require.NoError(svr.Start(ctx))
	require.NotNil(svr.Ap())

	// create client
	cfg.Network.BootstrapNodes = []string{svr.P2p().Self().String()}
	cli := network.NewOverlay(&cfg.Network)
	require.NotNil(cli)
	require.NoError(cli.Start(ctx))

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	from := testutil.ConstructAddress(fromPubKey, fromPrivKey)
	to := testutil.ConstructAddress(toPubKey, toPrivKey)

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		return len(svr.P2p().GetPeers()) == 1 && len(cli.GetPeers()) == 1, nil
	}))

	// Create three valid actions from "from" to "to"
	tsf1, _ := signedTransfer(from, to, uint64(1), big.NewInt(1))
	vote2, _ := signedVote(from, from, uint64(2))
	tsf3, _ := signedTransfer(from, to, uint64(3), big.NewInt(3))
	// Create three invalid actions from "from" to "to"
	// Existed Vote
	vote4, _ := signedVote(from, from, uint64(2))
	// Coinbase Transfer
	tsf5, _ := signedTransfer(from, to, uint64(5), big.NewInt(5))
	tsf5.IsCoinbase = true
	// Unsigned Vote
	vote6, _ := action.NewVote(uint64(6), from.RawAddress, from.RawAddress)

	require.NoError(cli.Broadcast(&pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf1.ConvertToTransferPb()}}))
	require.NoError(cli.Broadcast(&pb.ActionPb{Action: &pb.ActionPb_Vote{vote2.ConvertToVotePb()}}))
	require.NoError(cli.Broadcast(&pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf3.ConvertToTransferPb()}}))
	require.NoError(cli.Broadcast(&pb.ActionPb{Action: &pb.ActionPb_Vote{vote4.ConvertToVotePb()}}))
	require.NoError(cli.Broadcast(&pb.ActionPb{Action: &pb.ActionPb_Transfer{tsf5.ConvertToTransferPb()}}))
	require.NoError(cli.Broadcast(&pb.ActionPb{Action: &pb.ActionPb_Vote{vote6.ConvertToVotePb()}}))

	// Wait until server receives all the transfers
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		transfers, votes := svr.Ap().PickActs()
		// 2 valid transfers and 1 valid vote
		return len(transfers) == 2 && len(votes) == 1, nil
	}))
}

func TestPressureActPool(t *testing.T) {
	require := require.New(t)

	testutil.CleanupPath(t, testTriePath)
	testutil.CleanupPath(t, testDBPath)

	cfg, err := newActPoolConfig()
	require.NoError(err)

	blockchain.Gen.TotalSupply = uint64(50 << 22)
	blockchain.Gen.BlockReward = uint64(0)

	// create server
	ctx := context.Background()
	svr := itx.NewServer(cfg)
	require.Nil(svr.Start(ctx))
	require.NotNil(svr.Ap())

	// create client
	cfg.Network.BootstrapNodes = []string{svr.P2p().Self().String()}
	cli := network.NewOverlay(&cfg.Network)
	require.NotNil(cli)
	require.Nil(cli.Start(ctx))

	defer func() {
		require.Nil(cli.Stop(ctx))
		require.Nil(svr.Stop(ctx))
		testutil.CleanupPath(t, testTriePath)
		testutil.CleanupPath(t, testDBPath)
	}()

	from := testutil.ConstructAddress(fromPubKey, fromPrivKey)
	to := testutil.ConstructAddress(toPubKey, toPrivKey)

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		return len(svr.P2p().GetPeers()) == 1 && len(cli.GetPeers()) == 1, nil
	}))

	require.Nil(err)
	for i := 1; i <= 1000; i++ {
		tsf, _ := signedTransfer(from, to, uint64(i), big.NewInt(int64(i)))
		require.NoError(cli.Broadcast(&pb.ActionPb{Action: &pb.ActionPb_Transfer{Transfer: tsf.ConvertToTransferPb()}}))
	}

	// Wait until committed blocks contain all broadcasted actions
	err = testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		transfers, _ := svr.Ap().PickActs()
		return len(transfers) == 1000, nil
	})
	require.Nil(err)
}

// Helper function to return a signed transfer
func signedTransfer(
	sender *iotxaddress.Address,
	recipient *iotxaddress.Address,
	nonce uint64,
	amount *big.Int,
) (*action.Transfer, error) {
	transfer, err := action.NewTransfer(nonce, amount, sender.RawAddress, recipient.RawAddress)
	if err != nil {
		return nil, err
	}
	return transfer.Sign(sender)
}

// Helper function to return a signed vote
func signedVote(voter *iotxaddress.Address, votee *iotxaddress.Address, nonce uint64) (*action.Vote, error) {
	vote, err := action.NewVote(nonce, voter.RawAddress, votee.RawAddress)
	if err != nil {
		return nil, err
	}
	return vote.Sign(voter)
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

	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(addr.PublicKey)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(addr.PrivateKey)
	return &cfg, nil
}
