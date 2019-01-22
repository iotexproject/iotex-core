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
	"os"
	"path"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	exp "github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestTwoChains(t *testing.T) {
	dir := os.TempDir()
	cleanDB := func() {
		testutil.CleanupPath(t, path.Join(dir, "./trie.db"))
		testutil.CleanupPath(t, path.Join(dir, "./chain.db"))
		testutil.CleanupPath(t, path.Join(dir, "./chain-2-trie.db"))
		testutil.CleanupPath(t, path.Join(dir, "./chain-2-chain.db"))
	}

	cleanDB()

	cfg := config.Default
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Consensus.BlockCreationInterval = time.Second
	cfg.Chain.ProducerPrivKey = "925f0c9e4b6f6d92f2961d01aff6204c44d73c0b9d0da188582932d4fcad0d8ee8c66600"
	cfg.Chain.ProducerPubKey = "336eb60a5741f585a8e81de64e071327a3b96c15af4af5723598a07b6121e8e813bbd0056ba71ae29c0d64252e913f60afaeb11059908b81ff27cbfa327fd371d35f5ec0cbc01705"
	cfg.Chain.TrieDBPath = path.Join(dir, "./trie.db")
	cfg.Chain.ChainDBPath = path.Join(dir, "./chain.db")
	cfg.Chain.EnableSubChainStartInGenesis = true
	cfg.Chain.EnableIndex = true
	cfg.Chain.EnableAsyncIndexWrite = true
	cfg.Explorer.Enabled = true
	cfg.Explorer.Port = testutil.RandomPort()
	cfg.Network.Port = testutil.RandomPort()

	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, svr.Start(ctx))
	defer func() {
		cleanDB()
		require.NoError(t, svr.Stop(ctx))
	}()

	sk1, err := keypair.DecodePrivateKey(cfg.Chain.ProducerPrivKey)
	require.NoError(t, err)
	pk1, err := crypto.EC283.NewPubKey(sk1)
	require.NoError(t, err)
	pkHash1 := keypair.HashPubKey(pk1)
	addr1 := address.New(pkHash1[:])
	sk2, err := keypair.DecodePrivateKey("574f3b95c1afac4c5541ce705654bd92028e6b06bc07655647dd2637528dd98976f0c401")
	require.NoError(t, err)
	pk2, err := crypto.EC283.NewPubKey(sk2)
	require.NoError(t, err)
	pkHash2 := keypair.HashPubKey(pk2)
	addr2 := address.New(pkHash2[:])

	mainChainClient := exp.NewExplorerProxy(
		fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(cfg.Chain.ID).Explorer().Port()),
	)

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

	require.NoError(t, testutil.WaitUntil(time.Second, 10*time.Second, func() (bool, error) {
		balanceStr, err := mainChainClient.GetAddressBalance(addr1.Bech32())
		if err != nil {
			return false, err
		}
		balance, ok := big.NewInt(0).SetString(balanceStr, 10)
		if !ok {
			return false, errors.New("error when converting balance string to big int")
		}
		if balance.Cmp(big.NewInt(0).Mul(big.NewInt(1), big.NewInt(blockchain.Iotx))) < 0 {
			log.L().Info("Balance is not enough yet.", zap.String("balance", balance.String()))
			return false, nil
		}
		log.L().Info("Balance is already enough.", zap.String("balance", balance.String()))
		return true, nil
	}))

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

	details, err := mainChainClient.GetAddressDetails(addr1.Bech32())
	require.NoError(t, err)
	createDeposit := action.NewCreateDeposit(
		uint64(details.Nonce)+1,
		2,
		big.NewInt(0).Mul(big.NewInt(1), big.NewInt(blockchain.Iotx)),
		addr1.Bech32(),
		addr2.Bech32(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(createDeposit).
		SetNonce(uint64(details.Nonce) + 1).
		SetDestinationAddress(addr2.Bech32()).
		SetGasLimit(testutil.TestGasLimit).Build()
	selp, err := action.Sign(elp, addr1.Bech32(), sk1)
	require.NoError(t, err)

	createRes, err := mainChainClient.CreateDeposit(explorer.CreateDepositRequest{
		Version:      int64(createDeposit.Version()),
		Nonce:        int64(createDeposit.Nonce()),
		ChainID:      int64(createDeposit.ChainID()),
		Sender:       createDeposit.Sender(),
		SenderPubKey: keypair.EncodePublicKey(createDeposit.SenderPublicKey()),
		Recipient:    createDeposit.Recipient(),
		Amount:       createDeposit.Amount().String(),
		Signature:    hex.EncodeToString(selp.Signature()),
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

	details, err = subChainClient.GetAddressDetails(addr2.Bech32())
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
		addr1.Bech32(),
		addr2.Bech32(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(settleDeposit).
		SetNonce(nonce).
		SetDestinationAddress(addr2.Bech32()).
		SetGasLimit(testutil.TestGasLimit).Build()
	selp, err = action.Sign(elp, addr1.Bech32(), sk1)
	require.NoError(t, err)

	settleRes, err := subChainClient.SettleDeposit(explorer.SettleDepositRequest{
		Version:      int64(settleDeposit.Version()),
		Nonce:        int64(settleDeposit.Nonce()),
		Sender:       settleDeposit.Sender(),
		SenderPubKey: keypair.EncodePublicKey(settleDeposit.SenderPublicKey()),
		Recipient:    settleDeposit.Recipient(),
		Amount:       settleDeposit.Amount().String(),
		Index:        int64(index),
		Signature:    hex.EncodeToString(selp.Signature()),
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
