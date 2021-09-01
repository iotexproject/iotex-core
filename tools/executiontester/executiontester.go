// Copyright (c) 2019 IoTeX Foundation
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

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/tools/executiontester/assetcontract"
	"github.com/iotexproject/iotex-core/tools/executiontester/blockchain"
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
	cfg.Genesis.BlockInterval = 2 * time.Second
	cfg.ActPool.MinGasPriceStr = big.NewInt(0).String()
	itxsvr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to start itxServer.", zap.Error(err))
	}
	go itx.StartServer(ctx, itxsvr, probe.New(7799), cfg)

	time.Sleep(2 * time.Second)

	// Deploy contracts
	ret, err := assetcontract.StartContracts(cfg)
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

	if _, err := ret.FpToken.CreateToken(assetID, debtorAddr, creditorAddr, total, risk, open, exp); err != nil {
		log.L().Fatal("Failed to create fp token", zap.Error(err))
	}
	// create fpToken with assetID
	contractAddr, err := ret.FpToken.TokenAddress(assetID)
	if err != nil {
		log.L().Fatal("Failed to get token contract address", zap.Error(err))
	}
	tokenID := assetcontract.GenerateAssetID()
	// create erc721 token with tokenID
	if _, err := ret.Erc721Token.CreateToken(tokenID, creditorAddr); err != nil {
		log.L().Fatal("Failed to create erc721 token", zap.Error(err))
	}
	// Transfer fp token
	if _, err := ret.FpToken.Transfer(contractAddr, debtorAddr, debtorPriKey, creditorAddr, total); err != nil {
		log.L().Fatal("Failed to transfer total amount from debtor to creditor", zap.Error(err))
	}
	// Transfer erc721 token
	_, err = ret.Erc721Token.Transfer(ret.Erc721Token.Address(), creditorAddr, creditorPriKey, debtorAddr, tokenID)
	if err != nil {
		log.L().Fatal("Failed to transfer 1 token from creditor to debtor", zap.Error(err))
	}

	if _, err := ret.FpToken.RiskLock(contractAddr, creditorAddr, creditorPriKey, risk); err != nil {
		log.L().Fatal("Failed to transfer amount of risk from creditor to contract", zap.Error(err))
	}

	debtorBalance, err := ret.FpToken.ReadValue(contractAddr, blockchain.BalanceOf, debtorAddr)
	if err != nil {
		log.L().Fatal("Failed to get debtor's asset balance.", zap.Error(err))
	}
	log.L().Info("Debtor's asset balance: ", zap.Int64("balance", debtorBalance))

	creditorBalance, err := ret.FpToken.ReadValue(contractAddr, blockchain.BalanceOf, creditorAddr)
	if err != nil {
		log.L().Fatal("Failed to get creditor's asset balance.", zap.Error(err))
	}
	log.L().Info("Creditor's asset balance: ", zap.Int64("balance", creditorBalance))

	if debtorBalance+creditorBalance != total-risk {
		log.L().Fatal("Sum of balance is incorrect.")
	}

	log.L().Info("Fp token transfer test pass!")
	// get debtor balance,should be 1
	debtorBalance, err = ret.Erc721Token.ReadValue(ret.Erc721Token.Address(), blockchain.BalanceOf, debtorAddr)
	if err != nil {
		log.L().Fatal("Failed to get erc721 debtor's asset balance.", zap.Error(err))
	}
	log.L().Info("erc721 Debtor's asset balance: ", zap.Int64("balance", debtorBalance))
	// get creditor balance,should be 0
	creditorBalance, err = ret.Erc721Token.ReadValue(ret.Erc721Token.Address(), blockchain.BalanceOf, creditorAddr)
	if err != nil {
		log.L().Fatal("Failed to get erc721 creditor's asset balance.", zap.Error(err))
	}
	log.L().Info("erc721 Creditor's asset balance: ", zap.Int64("balance", creditorBalance))
	if (debtorBalance) != 1 && (creditorBalance != 0) {
		log.L().Fatal("erc721 balance is incorrect.")
	}
	log.L().Info("erc721 token transfer test pass!")

	// test ArrayDeletePassing func
	testArrayDeletePassing(ret.ArrDeletePassing)
	// test ArrayString func
	testArrayString(ret.ArrString)
}
func testArrayString(arr blockchain.ArrayString) {
	ret, err := arr.GetString()
	if err != nil {
		log.L().Fatal("Failed to call array-of-strings Func", zap.Error(err))
	}
	expected := "bye"
	if ret != expected {
		log.L().Fatal("one return is incorrect:", zap.String("string", ret))
	}
	log.L().Info("array-of-strings test pass!")
}
func testArrayDeletePassing(arr blockchain.ArrayDeletePassing) {
	ret, err := arr.GetArray()
	if err != nil {
		log.L().Fatal("Failed to call array-delete-passing GetArray Func", zap.Error(err))
	}
	expected := []*big.Int{big.NewInt(100), big.NewInt(200), big.NewInt(0), big.NewInt(400), big.NewInt(500)}

	for i, v := range ret {
		if v.Cmp(expected[i]) != 0 {
			log.L().Fatal("one return is incorrect:", zap.Int("array", i))
		}
	}

	retNum, err := arr.GetNum()
	if err != nil {
		log.L().Fatal("Failed to call array-delete-passing Func", zap.Error(err))
	}
	expectedNum := int64(10)
	if retNum != expectedNum {
		log.L().Fatal("one return is incorrect:", zap.Int64("string", retNum))
	}
	log.L().Info("array-delete-passing test pass!")
}
func createAccount() (string, string, string, error) {
	priKey, err := crypto.GenerateKey()
	if err != nil {
		return "", "", "", err
	}
	pubKey := priKey.PublicKey()
	addr := pubKey.Address()
	if addr == nil {
		return "", "", "", errors.New("failed to get address")
	}
	return pubKey.HexString(), priKey.HexString(), addr.String(), nil
}
