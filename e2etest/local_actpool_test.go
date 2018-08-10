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
	fromPubKey  = "ff613a9da51bad8da68c3d33d376600bb70fc336b4e2930946628110c9530618c19d6301d4a1c6f423fc227127494f258d352f086193ff27f9da83326a90df34adf0362988c95403"
	fromPrivKey = "13da3b1704c8a9a373c0e8166ac5690ddb05292372953e8f8c04a9bb75214ad321655401"
	// Recipient's public/private key pair
	toPubKey  = "934bb147113945418d572c63b7840768b5d540cded60e799674b7e24a2bff5ad3ec492068722e18053f18d270bcfe8d299a1cee36e758996b7dfb72a0617f60bdbb16f64062aca06"
	toPrivKey = "c40f03d0023be07a8d430bed0b825f01c5e480d6985001f337e88f6a9c9df489f7a87101"
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

	require.NoError(cli.Broadcast(tsf1.ConvertToActionPb()))
	require.NoError(cli.Broadcast(vote2.ConvertToActionPb()))
	require.NoError(cli.Broadcast(tsf3.ConvertToActionPb()))
	require.NoError(cli.Broadcast(vote4.ConvertToActionPb()))
	require.NoError(cli.Broadcast(tsf5.ConvertToActionPb()))
	require.NoError(cli.Broadcast(vote6.ConvertToActionPb()))

	// Wait until server receives all the transfers
	require.NoError(testutil.WaitUntil(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		transfers, votes := svr.ActionPool().PickActs()
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
		tsf, _ := signedTransfer(from, to, uint64(i), big.NewInt(int64(i)))
		require.NoError(cli.Broadcast(tsf.ConvertToActionPb()))
	}

	// Wait until committed blocks contain all broadcasted actions
	err = testutil.WaitUntil(10*time.Millisecond, 10*time.Second, func() (bool, error) {
		transfers, _ := svr.ActionPool().PickActs()
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
	cfg.Explorer.Port = 0

	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		return nil, err
	}
	cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(addr.PublicKey)
	cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(addr.PrivateKey)
	return &cfg, nil
}
