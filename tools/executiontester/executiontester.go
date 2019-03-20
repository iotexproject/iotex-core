// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"strconv"
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
	itxsvr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to start itxServer.", zap.Error(err))
	}
	go itx.StartServer(ctx, itxsvr, probe.New(7799), cfg)

	time.Sleep(2 * time.Second)

	// Deploy contracts
	chainEndPoint := "127.0.0.1:" + strconv.Itoa(cfg.API.Port)
	fpToken, _, err := assetcontract.StartContracts(chainEndPoint)
	if err != nil {
		log.L().Fatal("Failed to deploy contracts.", zap.Error(err))
	}

	// Create two accounts
	//debtorPubKey, debtorPriKey, debtorAddr, err := createAccount()
	//if err != nil {
	//	log.L().Fatal("Failed to create account.", zap.Error(err))
	//}
	//_, _, creditorAddr, err := createAccount()
	//if err != nil {
	//	log.L().Fatal("Failed to create account.", zap.Error(err))
	//}
	debtorPubKey := "045c140600209d9d0860bb88aeb3c25d60058de3edaefef2b3c7ec601aa98cf3c6c9b8117354f99219779cbcc8cd78534ddc34bfb253920eb138994b902b9ec19d"
	debtorPriKey := "712353f1e18b3901ce2f7f1b2b045af621d7c2b6d529385849385f24f5d9953f"
	debtorAddr := "io1crl7nek497h5lmj8qy97dxlfegmrev7cqwtlup"
	creditorAddr := "io1ygj3q0xcm3rm639k75u3wqhtcts7t8w2gxj2s4"


	// Create fp token
	//assetID := assetcontract.GenerateAssetID()
	//open := strconv.Itoa(int(time.Now().UnixNano() / 1e6))
	//exp := strconv.Itoa(int(time.Now().UnixNano()/1e6 + 1000000))
	assetID := "2019031938960305"
	open := "1553060435132"
	exp := "1553061435132"


	if _, err := fpToken.CreateToken(assetID, debtorAddr, creditorAddr, total, risk, open, exp); err != nil {
		log.L().Fatal("Failed to create fp token", zap.Error(err))
	}

	contractAddr, err := fpToken.TokenAddress(assetID)
	if err != nil {
		log.L().Fatal("Failed to get token contract address", zap.Error(err))
	}
	log.L().Error("fp token contract address", zap.String("address", contractAddr))

	log.L().Error("Start here.................")

	// Transfer fp token
	if _, err := fpToken.Transfer(contractAddr, debtorAddr, debtorPubKey, debtorPriKey, creditorAddr, transfer); err != nil {
		log.L().Fatal("Failed to transfer fp token from debtor to creditor", zap.Error(err))
	}

	log.L().Error("End here...................")

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

	if debtorBalance+creditorBalance != total {
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
