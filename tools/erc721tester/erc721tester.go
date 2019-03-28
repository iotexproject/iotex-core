// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"context"
	"math/big"
	"time"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/tools/erc721tester/assetcontract"
)

func main() {
	ctx := context.Background()
	// Start iotex-server
	cfg := config.Default
	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.ActionGasLimit = 10000000
	cfg.Genesis.BlockInterval = 2 * time.Second
	cfg.ActPool.MinGasPriceStr = big.NewInt(0).String()
	itxsvr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to start itxServer.", zap.Error(err))
	}
	go itx.StartServer(ctx, itxsvr, probe.New(7799), cfg)

	time.Sleep(2 * time.Second)

	// Deploy contracts
	erc721Token, err := assetcontract.StartContracts(cfg)
	if err != nil {
		log.L().Fatal("Failed to deploy contracts.", zap.Error(err))
	}
	// Create two accounts
	_, _, debtorAddr, err := createAccount()
	if err != nil {
		log.L().Fatal("Failed to create account.", zap.Error(err))
	}
	_, creditorPriv, creditorAddr, err := createAccount()
	if err != nil {
		log.L().Fatal("Failed to create account.", zap.Error(err))
	}

	// Create erc721 token
	tokenID := assetcontract.GenerateAssetID()
	if _, err := erc721Token.CreateToken(tokenID,creditorAddr); err != nil {
		log.L().Fatal("Failed to create token", zap.Error(err))
	}

	creditorBalance, err := erc721Token.ReadValue(erc721Token.Address(), "70a08231", creditorAddr)
	if err != nil {
		log.L().Fatal("Failed to get creditor's asset balance.", zap.Error(err))
	}
	log.L().Info("Creditor's asset balance: ", zap.Int64("balance", creditorBalance))


	//// Transfer erc721 token
	if _, err := erc721Token.Transfer(erc721Token.Address(), creditorAddr, creditorPriv, debtorAddr, tokenID); err != nil {
		log.L().Fatal("Failed to transfer 1 token from creditor to debtor", zap.Error(err))
	}

	debtorBalance, err := erc721Token.ReadValue(erc721Token.Address(), "70a08231", debtorAddr)
	if err != nil {
		log.L().Fatal("Failed to get debtor's asset balance.", zap.Error(err))
	}
	log.L().Info("Debtor's asset balance: ", zap.Int64("balance", debtorBalance))

	creditorBalance, err = erc721Token.ReadValue(erc721Token.Address(), "70a08231", creditorAddr)
	if err != nil {
		log.L().Fatal("Failed to get debtor's asset balance.", zap.Error(err))
	}
	log.L().Info("Creditor's asset balance: ", zap.Int64("balance", creditorBalance))

	log.L().Info("Token transfer test pass!")

	<-ctx.Done()
}

func createAccount() (string, string, string, error) {
	priKey, err := keypair.GenerateKey()
	if err != nil {
		return "", "", "", err
	}
	pubKey := priKey.PublicKey()
	addr, err := address.FromBytes(pubKey.Hash())
	if err != nil {
		return "", "", "", err
	}
	return pubKey.HexString(), priKey.HexString(), addr.String(), nil
}
