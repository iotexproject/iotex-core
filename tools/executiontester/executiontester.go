// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/server/itx"
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
	cfg.Genesis.ActionGasLimit = 10000000
	cfg.Genesis.BlockInterval = 2
	itxsvr, err := itx.NewServer(cfg)
	if err != nil {
		log.L().Fatal("Failed to start itxServer.", zap.Error(err))
	}
	go itx.StartServer(ctx, itxsvr, probe.New(7799), cfg)

	time.Sleep(2 * time.Second)

	// Deploy contracts
	fpToken, _, err := startContracts()
	if err != nil {
		log.L().Fatal("Failed to deploy contracts.", zap.Error(err))
	}

	// Create two accounts
	debtorPubKey, debtorPriKey, debtorAddr, err := createAccount()
	if err != nil {
		log.L().Fatal("Failed to create account.", zap.Error(err))
	}
	_, _, creditorAddr, err := createAccount()
	if err != nil {
		log.L().Fatal("Failed to create account.", zap.Error(err))
	}

	// Create fp token
	assetID := generateAssetID()
	open := strconv.Itoa(int(time.Now().UnixNano() / 1e6))
	exp := strconv.Itoa(int(time.Now().UnixNano()/1e6 + 1000000))

	if _, err := fpToken.CreateToken(assetID, debtorAddr, creditorAddr, total, risk, open, exp); err != nil {
		log.L().Fatal("Failed to create fp token", zap.Error(err))
	}

	contractAddr, err := fpToken.TokenAddress(assetID)
	if err != nil {
		log.L().Fatal("Failed to get token contract address", zap.Error(err))
	}

	// Transfer fp token
	if _, err := fpToken.Transfer(contractAddr, debtorAddr, debtorPubKey, debtorPriKey, creditorAddr, transfer); err != nil {
		log.L().Fatal("Failed to transfer fp token from debtor to creditor", zap.Error(err))
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

	if debtorBalance+creditorBalance != total {
		log.L().Fatal("Sum of balance is incorrect.")
	}

	log.L().Info("Fp token transfer test pass!")
}

func startContracts() (blockchain.FpToken, blockchain.StableToken, error) {
	// deploy allowance sheet
	allowance, err := deployContract(blockchain.AllowanceSheetBinary)
	if err != nil {
		return nil, nil, err
	}
	// deploy balance sheet
	balance, err := deployContract(blockchain.BalanceSheetBinary)
	if err != nil {
		return nil, nil, err
	}
	// deploy registry
	reg, err := deployContract(blockchain.RegistryBinary)
	if err != nil {
		return nil, nil, err
	}
	// deploy global pause
	pause, err := deployContract(blockchain.GlobalPauseBinary)
	if err != nil {
		return nil, nil, err
	}
	// deploy stable token
	stable, err := deployContract(blockchain.StableTokenBinary)
	if err != nil {
		return nil, nil, err
	}

	// create stable token
	// TODO: query total supply and call stbToken.SetTotal()
	stbToken := blockchain.NewStableToken(blockchain.ChainEndpoint).
		SetAllowance(allowance).
		SetBalance(balance).
		SetRegistry(reg).
		SetPause(pause).
		SetStable(stable)
	stbToken.SetOwner(blockchain.Producer, blockchain.ProducerPubKey, blockchain.ProducerPrivKey)

	// stable token set-up
	if err := stbToken.Start(); err != nil {
		return nil, nil, err
	}

	// deploy fp token
	fpReg, err := deployContract(blockchain.FpRegistryBinary)
	if err != nil {
		return nil, nil, err
	}
	cdp, err := deployContract(blockchain.CdpManageBinary)
	if err != nil {
		return nil, nil, err
	}
	manage, err := deployContract(blockchain.ManageBinary)
	if err != nil {
		return nil, nil, err
	}
	proxy, err := deployContract(blockchain.ManageProxyBinary)
	if err != nil {
		return nil, nil, err
	}
	eap, err := deployContract(blockchain.EapStorageBinary)
	if err != nil {
		return nil, nil, err
	}
	riskLock, err := deployContract(blockchain.TokenRiskLockBinary)
	if err != nil {
		return nil, nil, err
	}

	// create fp token
	fpToken := blockchain.NewFpToken(blockchain.ChainEndpoint).
		SetManagement(manage).
		SetManagementProxy(proxy).
		SetEapStorage(eap).
		SetRiskLock(riskLock).
		SetRegistry(fpReg).
		SetCdpManager(cdp).
		SetStableToken(stable)
	fpToken.SetOwner(blockchain.Producer, blockchain.ProducerPubKey, blockchain.ProducerPrivKey)

	// fp token set-up
	if err := fpToken.Start(); err != nil {
		return nil, nil, err
	}

	return fpToken, stbToken, nil
}

func deployContract(code string) (string, error) {
	// deploy the contract
	contract := blockchain.NewContract(blockchain.ChainEndpoint)
	h, err := contract.
		SetExecutor(blockchain.Producer).
		SetPubKey(blockchain.ProducerPubKey).
		SetPrvKey(blockchain.ProducerPrivKey).
		Deploy(code)
	if err != nil {
		return "", errors.Wrapf(err, "failed to deploy contract, txhash = %s", h)
	}

	time.Sleep(time.Second * 3)
	receipt, err := contract.CheckCallResult(h)
	if err != nil {
		return "", errors.Wrapf(err, "failed to deploy contract, txhash = %s", h)
	}
	return receipt.ContractAddress, nil
}

func generateAssetID() string {
	for {
		id := strconv.Itoa(rand.Int())
		if len(id) >= 8 {
			// attach 8-digit YYYYMMDD at front
			t := time.Now().Format(time.RFC3339)
			t = strings.Replace(t, "-", "", -1)
			return t[:8] + id[:8]
		}
	}
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
