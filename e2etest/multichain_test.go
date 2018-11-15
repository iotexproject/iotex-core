// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	exp "github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestTwoChains(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping the two chains test in short mode.")
	}

	cleanDB := func() {
		testutil.CleanupPath(t, "./trie.db")
		testutil.CleanupPath(t, "./chain.db")
		testutil.CleanupPath(t, "./chain-2-trie.db")
		testutil.CleanupPath(t, "./chain-2-chain.db")
	}

	cleanDB()

	cfg := config.Default
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Consensus.BlockCreationInterval = time.Second
	cfg.Chain.ProducerPrivKey = "925f0c9e4b6f6d92f2961d01aff6204c44d73c0b9d0da188582932d4fcad0d8ee8c66600"
	cfg.Chain.ProducerPubKey = "336eb60a5741f585a8e81de64e071327a3b96c15af4af5723598a07b6121e8e813bbd0056ba71ae29c0d64252e913f60afaeb11059908b81ff27cbfa327fd371d35f5ec0cbc01705"
	cfg.Chain.TrieDBPath = "./trie.db"
	cfg.Chain.ChainDBPath = "./chain.db"
	cfg.Chain.EnableSubChainStartInGenesis = true
	cfg.Explorer.Enabled = true

	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, svr.Start(ctx))
	defer func() {
		cleanDB()
		require.NoError(t, svr.Stop(ctx))
	}()

	time.Sleep(time.Second)

	sk, err := keypair.DecodePrivateKey("d2df3528ff384d41cc9688c354cd301a09f91d95582eb8034a6eff140e7539cb17b53401")
	sk1, err := keypair.DecodePrivateKey("574f3b95c1afac4c5541ce705654bd92028e6b06bc07655647dd2637528dd98976f0c401")
	require.NoError(t, err)
	pk, err := crypto.EC283.NewPubKey(sk)
	require.NoError(t, err)
	pkHash := keypair.HashPubKey(pk)

	pk1, err := crypto.EC283.NewPubKey(sk1)
	require.NoError(t, err)
	pkHash1 := keypair.HashPubKey(pk1)
	addr1 := address.New(1, pkHash[:])
	addr2 := address.New(2, pkHash1[:])

	mainChainClient := exp.NewExplorerProxy(
		fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(cfg.Chain.ID).Explorer().Port()),
	)

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

	require.NoError(t, testutil.WaitUntil(time.Second, 10*time.Second, func() (bool, error) {
		balanceStr, err := mainChainClient.GetAddressBalance(addr1.IotxAddress())
		if err != nil {
			return false, err
		}
		balance, ok := big.NewInt(0).SetString(balanceStr, 10)
		if !ok {
			return false, errors.New("error when converting balance string to big int")
		}
		if balance.Cmp(big.NewInt(0).Mul(big.NewInt(1), big.NewInt(blockchain.Iotx))) < 0 {
			logger.Info().Str("balance", balance.String()).Msg("balance is not enough yet")
			return false, nil
		}
		logger.Info().Str("balance", balance.String()).Msg("balance is already enough")
		return true, nil
	}))

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

	details, err := mainChainClient.GetAddressDetails(addr1.IotxAddress())
	require.NoError(t, err)
	createDeposit := action.NewCreateDeposit(
		uint64(details.Nonce)+1,
		big.NewInt(0).Mul(big.NewInt(1), big.NewInt(blockchain.Iotx)),
		addr1.IotxAddress(),
		addr2.IotxAddress(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	require.NoError(t, action.Sign(createDeposit, sk))

	createRes, err := mainChainClient.CreateDeposit(explorer.CreateDepositRequest{
		Version:      int64(createDeposit.Version()),
		Nonce:        int64(createDeposit.Nonce()),
		Sender:       createDeposit.Sender(),
		SenderPubKey: keypair.EncodePublicKey(createDeposit.SenderPublicKey()),
		Recipient:    createDeposit.Recipient(),
		Amount:       createDeposit.Amount().String(),
		Signature:    hex.EncodeToString(createDeposit.Signature()),
		GasLimit:     int64(createDeposit.GasLimit()),
		GasPrice:     createDeposit.GasPrice().String(),
	})
	require.NoError(t, err)

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		_, err := mainChainClient.GetReceiptByExecutionID(createRes.Hash)
		return err == nil, nil
	}))

	cd1, err := mainChainClient.GetCreateDeposit(createRes.Hash)
	require.NoError(t, err)
	cds, err := mainChainClient.GetCreateDepositsByAddress(createDeposit.Sender(), 0, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(cds))
	assert.Equal(t, cd1, cds[0])

	receipt, err := mainChainClient.GetReceiptByExecutionID(createRes.Hash)
	require.NoError(t, err)
	value, err := hex.DecodeString(receipt.ReturnValue)
	require.NoError(t, err)
	index := enc.MachineEndian.Uint64(value)

	subChainClient := exp.NewExplorerProxy(
		fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(cfg.Chain.ID).Explorer().Port()+1),
	)

	details, err = subChainClient.GetAddressDetails(addr2.IotxAddress())
	var nonce uint64
	if err != nil {
		nonce = 1
	} else {
		nonce = uint64(details.PendingNonce)
	}
	settleDeposit := action.NewSettleDeposit(
		nonce,
		big.NewInt(0).Mul(big.NewInt(1), big.NewInt(blockchain.Iotx)),
		index,
		addr1.IotxAddress(),
		addr2.IotxAddress(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	require.NoError(t, action.Sign(settleDeposit, sk))

	settleRes, err := subChainClient.SettleDeposit(explorer.SettleDepositRequest{
		Version:      int64(settleDeposit.Version()),
		Nonce:        int64(settleDeposit.Nonce()),
		Sender:       settleDeposit.Sender(),
		SenderPubKey: keypair.EncodePublicKey(settleDeposit.SenderPublicKey()),
		Recipient:    settleDeposit.Recipient(),
		Amount:       settleDeposit.Amount().String(),
		Index:        int64(index),
		Signature:    hex.EncodeToString(settleDeposit.Signature()),
		GasLimit:     int64(settleDeposit.GasLimit()),
		GasPrice:     settleDeposit.GasPrice().String(),
	})
	require.NoError(t, err)

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		sd, err := subChainClient.GetSettleDeposit(settleRes.Hash)
		return err == nil && sd.IsPending == false, nil
	}))

	sd1, err := subChainClient.GetSettleDeposit(settleRes.Hash)
	require.NoError(t, err)
	sds, err := subChainClient.GetSettleDepositsByAddress(settleDeposit.Recipient(), 0, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(sds))
	assert.Equal(t, sd1, sds[0])
}
