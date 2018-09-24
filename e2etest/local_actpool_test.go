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
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	// Make sure the key pairs used here match the genesis block
	// Sender's public/private key pair
	fromPubKey  = "3f25b0312791c2c1a4c7f906455622bbafe2406232ec8734f559ce454fbaefd1fade8601c3e8c93a771f621ae8cbd34f6b4036e2a0715a1865fd1a4de5ece105d1579300eacaee01"
	fromPrivKey = "f1e4ff82b5bb480ebfdc89752e78beaf3a6adb3ffd8a3b5eaa09428839bd2478e4a56201"
	// Recipient's public/private key pair
	toPubKey  = "0b19932e0ea8553538ac9c0bec245c1b826f3342a5a79a63a03d637331656f57bde8cb0122bf9b2f0d6a1e4d50b9019b0ef99be21d858a20d7314e780199deb44f8dcc0704f78404"
	toPrivKey = "c3a6f7a3392a8e4e97d3dc3993b908abc35e7398548fb269a3fb67bc4a37e60270940f01"
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
	svr := itx.NewServer(cfg)
	require.NoError(svr.Start(ctx))
	require.NotNil(svr.ActionPool())

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

	from := testutil.ConstructAddress(fromPubKey, fromPrivKey)
	to := testutil.ConstructAddress(toPubKey, toPrivKey)

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		return len(svr.P2P().GetPeers()) == 1 && len(cli.GetPeers()) == 1, nil
	}))

	// Create three valid actions from "from" to "to"
	tsf1, _ := signedTransfer(from, to, uint64(1), big.NewInt(1), []byte{}, uint64(100000), big.NewInt(0))
	vote2, _ := signedVote(from, from, uint64(2), uint64(100000), big.NewInt(0))
	tsf3, _ := signedTransfer(from, to, uint64(3), big.NewInt(3), []byte{}, uint64(100000), big.NewInt(0))
	// Create contract
	exec4, _ := signedExecution(from, action.EmptyAddress, uint64(4), big.NewInt(0), uint64(120000), big.NewInt(10), []byte{})

	// Create three invalid actions from "from" to "to"
	// Existed Vote
	vote5, _ := signedVote(from, from, uint64(2), uint64(100000), big.NewInt(0))
	// Unsigned Vote
	vote6, _ := action.NewVote(uint64(7), from.RawAddress, from.RawAddress, uint64(100000), big.NewInt(10))

	require.NoError(cli.Broadcast(tsf1.ConvertToActionPb()))
	require.NoError(cli.Broadcast(vote2.ConvertToActionPb()))
	require.NoError(cli.Broadcast(tsf3.ConvertToActionPb()))
	require.NoError(cli.Broadcast(exec4.ConvertToActionPb()))
	require.NoError(cli.Broadcast(vote5.ConvertToActionPb()))
	require.NoError(cli.Broadcast(vote6.ConvertToActionPb()))

	// Wait until server receives all the transfers
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		transfers, votes, executions := svr.ActionPool().PickActs()
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
	svr := itx.NewServer(cfg)
	require.Nil(svr.Start(ctx))
	require.NotNil(svr.ActionPool())

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

	from := testutil.ConstructAddress(fromPubKey, fromPrivKey)
	to := testutil.ConstructAddress(toPubKey, toPrivKey)

	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		return len(svr.P2P().GetPeers()) == 1 && len(cli.GetPeers()) == 1, nil
	}))

	require.Nil(err)
	for i := 1; i <= 1000; i++ {
		tsf, _ := signedTransfer(from, to, uint64(i), big.NewInt(int64(i)), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(cli.Broadcast(tsf.ConvertToActionPb()))
	}

	// Wait until committed blocks contain all broadcasted actions
	err = testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		transfers, _, _ := svr.ActionPool().PickActs()
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
	payload []byte,
	gasLimit uint64,
	gasPrice *big.Int,
) (*action.Transfer, error) {
	transfer, err := action.NewTransfer(nonce, amount, sender.RawAddress, recipient.RawAddress, payload, gasLimit, gasPrice)
	if err != nil {
		return nil, err
	}
	if err := action.Sign(transfer, sender); err != nil {
		return nil, err
	}
	return transfer, nil
}

// Helper function to return a signed vote
func signedVote(voter *iotxaddress.Address, votee *iotxaddress.Address, nonce uint64, gasLimit uint64, gasPrice *big.Int) (*action.Vote, error) {
	vote, err := action.NewVote(nonce, voter.RawAddress, votee.RawAddress, gasLimit, gasPrice)
	if err != nil {
		return nil, err
	}
	if err := action.Sign(vote, voter); err != nil {
		return nil, err
	}
	return vote, err
}

// Helper function to return a signed execution
func signedExecution(executor *iotxaddress.Address, contractAddr string, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (*action.Execution, error) {
	execution, err := action.NewExecution(executor.RawAddress, contractAddr, nonce, amount, gasLimit, gasPrice, data)
	if err != nil {
		return nil, err
	}
	if err := action.Sign(execution, executor); err != nil {
		return nil, err
	}
	return execution, nil
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

	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(addr.PublicKey)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(addr.PrivateKey)
	return &cfg, nil
}
