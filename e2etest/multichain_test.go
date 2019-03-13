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
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	exp "github.com/iotexproject/iotex-core/explorer"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestTwoChains(t *testing.T) {
	t.Skip()

	testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
	testDBPath := testDBFile.Name()

	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Consensus.Scheme = config.StandaloneScheme
	cfg.Genesis.BlockInterval = time.Second
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(1).HexString()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Explorer.Enabled = true
	cfg.Explorer.Port = testutil.RandomPort()
	cfg.Network.Port = testutil.RandomPort()

	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, svr.Start(ctx))
	defer func() {
		require.NoError(t, svr.Stop(ctx))
	}()

	sk1, err := keypair.HexStringToPrivateKey(cfg.Chain.ProducerPrivKey)
	require.NoError(t, err)
	addr1, err := address.FromBytes(sk1.PublicKey().Hash())
	require.NoError(t, err)
	sk2, err := keypair.HexStringToPrivateKey("82a1556b2dbd0e3615e367edf5d3b90ce04346ec4d12ed71f67c70920ef9ac90")
	require.NoError(t, err)
	addr2, err := address.FromBytes(sk2.PublicKey().Hash())
	require.NoError(t, err)

	mainChainClient := exp.NewExplorerProxy(
		fmt.Sprintf("http://127.0.0.1:%d", svr.ChainService(cfg.Chain.ID).Explorer().Port()),
	)

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

	require.NoError(t, testutil.WaitUntil(time.Second, 10*time.Second, func() (bool, error) {
		balanceStr, err := mainChainClient.GetAddressBalance(addr1.String())
		if err != nil {
			return false, err
		}
		balance, ok := big.NewInt(0).SetString(balanceStr, 10)
		if !ok {
			return false, errors.New("error when converting balance string to big int")
		}
		if balance.Cmp(big.NewInt(0).Mul(big.NewInt(1), big.NewInt(unit.Iotx))) < 0 {
			log.L().Info("Balance is not enough yet.", zap.String("balance", balance.String()))
			return false, nil
		}
		log.L().Info("Balance is already enough.", zap.String("balance", balance.String()))
		return true, nil
	}))

	require.NoError(t, testutil.WaitUntil(time.Second, 20*time.Second, func() (bool, error) {
		return svr.ChainService(2) != nil, nil
	}))

	details, err := mainChainClient.GetAddressDetails(addr1.String())
	require.NoError(t, err)
	createDeposit := action.NewCreateDeposit(
		uint64(details.Nonce)+1,
		2,
		big.NewInt(0).Mul(big.NewInt(1), big.NewInt(unit.Iotx)),
		addr2.String(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetAction(createDeposit).
		SetNonce(uint64(details.Nonce) + 1).
		SetGasLimit(testutil.TestGasLimit).Build()
	selp, err := action.Sign(elp, sk1)
	require.NoError(t, err)

	createRes, err := mainChainClient.CreateDeposit(explorer.CreateDepositRequest{
		Version:      int64(createDeposit.Version()),
		Nonce:        int64(createDeposit.Nonce()),
		ChainID:      int64(createDeposit.ChainID()),
		SenderPubKey: createDeposit.SenderPublicKey().HexString(),
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
	cds, err := mainChainClient.GetCreateDepositsByAddress(addr1.String(), 0, 1)
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

	details, err = subChainClient.GetAddressDetails(addr2.String())
	var nonce uint64
	if err != nil {
		nonce = 1
	} else {
		nonce = uint64(details.PendingNonce)
	}
	settleDeposit := action.NewSettleDeposit(
		nonce,
		big.NewInt(0).Mul(big.NewInt(1), big.NewInt(unit.Iotx)),
		index,
		addr2.String(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)
	bd = &action.EnvelopeBuilder{}
	elp = bd.SetAction(settleDeposit).
		SetNonce(nonce).
		SetGasLimit(testutil.TestGasLimit).Build()
	selp, err = action.Sign(elp, sk1)
	require.NoError(t, err)

	settleRes, err := subChainClient.SettleDeposit(explorer.SettleDepositRequest{
		Version:      int64(settleDeposit.Version()),
		Nonce:        int64(settleDeposit.Nonce()),
		SenderPubKey: settleDeposit.SenderPublicKey().HexString(),
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
