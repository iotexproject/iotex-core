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

	mainChainClient := exp.NewExplorerProxy(
		fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(cfg.Chain.ID).Explorer().Port()),
	)
	producerAddr, err := cfg.BlockchainAddress()
	require.NoError(t, err)
	require.NoError(t, testutil.WaitUntil(time.Second, 10*time.Second, func() (bool, error) {
		balanceStr, err := mainChainClient.GetAddressBalance(producerAddr.IotxAddress())
		if err != nil {
			return false, err
		}
		balance, ok := big.NewInt(0).SetString(balanceStr, 10)
		if !ok {
			return false, errors.New("error when converting balance string to big int")
		}
		if balance.Cmp(big.NewInt(0).Mul(big.NewInt(5), big.NewInt(blockchain.Iotx))) < 0 {
			logger.Info().Str("balance", balance.String()).Msg("balance is not enough yet")
			return false, nil
		}
		logger.Info().Str("balance", balance.String()).Msg("balance is already enough")
		return true, nil
	}))

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

	details, err := mainChainClient.GetAddressDetails(producerAddr.IotxAddress())
	require.NoError(t, err)
	deposit := action.NewCreateDeposit(
		uint64(details.Nonce)+1,
		big.NewInt(0).Mul(big.NewInt(5), big.NewInt(blockchain.Iotx)),
		producerAddr.IotxAddress(),
		address.New(2, producerAddr.Payload()).IotxAddress(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	_, sk, err := cfg.KeyPair()
	require.NoError(t, err)
	require.NoError(t, action.Sign(deposit, sk))

	createRes, err := mainChainClient.CreateDeposit(explorer.CreateDepositRequest{
		Version:      int64(deposit.Version()),
		Nonce:        int64(deposit.Nonce()),
		Sender:       deposit.Sender(),
		SenderPubKey: keypair.EncodePublicKey(deposit.SenderPublicKey()),
		Recipient:    deposit.Recipient(),
		Amount:       deposit.Amount().String(),
		Signature:    hex.EncodeToString(deposit.Signature()),
		GasLimit:     int64(deposit.GasLimit()),
		GasPrice:     deposit.GasPrice().String(),
	})
	require.NoError(t, err)

	require.NoError(t, testutil.WaitUntil(time.Second, 10*time.Second, func() (bool, error) {
		_, err := mainChainClient.GetReceiptByExecutionID(createRes.Hash)
		return err == nil, nil
	}))

	cd1, err := mainChainClient.GetCreateDeposit(createRes.Hash)
	require.NoError(t, err)
	cds, err := mainChainClient.GetCreateDepositsByAddress(deposit.Sender(), 0, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(cds))
	assert.Equal(t, cd1, cds[0])

	receipt, err := mainChainClient.GetReceiptByExecutionID(createRes.Hash)
	require.NoError(t, err)
	value, err := hex.DecodeString(receipt.ReturnValue)
	require.NoError(t, err)
	index := enc.MachineEndian.Uint64(value)

	subChainClient := exp.NewExplorerProxy(
		fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(2).Explorer().Port()),
	)
	settleRes, err := subChainClient.SettleDeposit(explorer.SettleDepositRequest{
		Version:      int64(deposit.Version()),
		Nonce:        int64(deposit.Nonce()),
		Sender:       deposit.Sender(),
		SenderPubKey: keypair.EncodePublicKey(deposit.SenderPublicKey()),
		Recipient:    deposit.Recipient(),
		Amount:       deposit.Amount().String(),
		Index:        int64(index),
		Signature:    hex.EncodeToString(deposit.Signature()),
		GasLimit:     int64(deposit.GasLimit()),
		GasPrice:     deposit.GasPrice().String(),
	})
	require.NoError(t, err)

	require.NoError(t, testutil.WaitUntil(time.Second, 10*time.Second, func() (bool, error) {
		sd, err := subChainClient.GetSettleDeposit(settleRes.Hash)
		return err == nil && sd.IsPending == false, nil
	}))

	sd1, err := subChainClient.GetSettleDeposit(settleRes.Hash)
	require.NoError(t, err)
	sds, err := subChainClient.GetSettleDepositsByAddress(deposit.Recipient(), 0, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(sds))
	assert.Equal(t, sd1, sds[0])
}
