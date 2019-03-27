// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"math/big"
	"time"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/tools/executiontester/assetcontract"
)

func main() {
	// total indicates the total amount value of a fp token
	var total int64
	// risk indicates the risk amount value of a fp token
	var risk int64
	// transfer indicates the transfer amount value of a fp token
	var transfer int64

	flag.Int64Var(&total, "total", 10000, "total amount value of a fp token")
	flag.Int64Var(&risk, "risk", 2000, "risk amount value of a fp token")
	flag.Int64Var(&transfer, "transfer", 1000, "transfer amount value of a fp token")
	flag.Parse()

	if risk > total {
		log.L().Fatal("risk amount cannot be greater than total amount")
	}

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
	fpToken, _, err := assetcontract.StartContracts(cfg)
	if err != nil {
		log.L().Fatal("Failed to deploy contracts.", zap.Error(err))
	}

	// Create two accounts
	_, debtorPriKey, debtorAddr, err := createAccount()
	if err != nil {
		log.L().Fatal("Failed to create account.", zap.Error(err))
	}
	_, creditorPriKey, creditorAddr, err := createAccount()
	if err != nil {
		log.L().Fatal("Failed to create account.", zap.Error(err))
	}

	// Create fp token
	assetID := assetcontract.GenerateAssetID()
	open := time.Now().Unix()
	exp := open + 100000

	if _, err := fpToken.CreateToken(assetID, debtorAddr, creditorAddr, total, risk, open, exp); err != nil {
		log.L().Fatal("Failed to create fp token", zap.Error(err))
	}

	contractAddr, err := fpToken.TokenAddress(assetID)
	if err != nil {
		log.L().Fatal("Failed to get token contract address", zap.Error(err))
	}

	// Transfer fp token
	if _, err := fpToken.Transfer(contractAddr, debtorAddr, debtorPriKey, creditorAddr, total); err != nil {
		log.L().Fatal("Failed to transfer total amount from debtor to creditor", zap.Error(err))
	}
	if _, err := fpToken.RiskLock(contractAddr, creditorAddr, creditorPriKey, risk); err != nil {
		log.L().Fatal("Failed to transfer amount of risk from creditor to contract", zap.Error(err))
	}

	debtorBalance, err := fpToken.ReadValue(contractAddr, "70a08231", debtorAddr)
	if err != nil {
		log.L().Fatal("Failed to get debtor's asset balance.", zap.Error(err))
	}
	log.L().Info("Debtor's asset balance: ", zap.Int64("balance", debtorBalance))

	creditorBalance, err := fpToken.ReadValue(contractAddr, "70a08231", creditorAddr)
	if err != nil {
		log.L().Fatal("Failed to get creditor's asset balance.", zap.Error(err))
	}
	log.L().Info("Creditor's asset balance: ", zap.Int64("balance", creditorBalance))

	if debtorBalance+creditorBalance != total-risk {
		log.L().Fatal("Sum of balance is incorrect.")
	}

	log.L().Info("Fp token transfer test pass!")
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
