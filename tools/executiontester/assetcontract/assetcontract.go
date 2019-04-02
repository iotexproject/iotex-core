package assetcontract

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/tools/executiontester/blockchain"
)

const (
	// ChainIP is the ip address of iotex api endpoint
	chainIP = "localhost"
)

// StartContracts deploys and starts fp token smart contract and stable token smart contract,erc721 token smart contract
func StartContracts(cfg config.Config) (fpToken blockchain.FpToken, stbToken blockchain.StableToken, erc721Token blockchain.Erc721Token, err error) {
	endpoint := chainIP + ":" + strconv.Itoa(cfg.API.Port)

	// deploy allowance sheet
	allowance, err := deployContract(blockchain.AllowanceSheetBinary, endpoint)
	if err != nil {
		return
	}
	// deploy balance sheet
	balance, err := deployContract(blockchain.BalanceSheetBinary, endpoint)
	if err != nil {
		return
	}
	// deploy registry
	reg, err := deployContract(blockchain.RegistryBinary, endpoint)
	if err != nil {
		return
	}
	// deploy global pause
	pause, err := deployContract(blockchain.GlobalPauseBinary, endpoint)
	if err != nil {
		return
	}
	// deploy stable token
	stable, err := deployContract(blockchain.StableTokenBinary, endpoint)
	if err != nil {
		return
	}

	// create stable token
	// TODO: query total supply and call stbToken.SetTotal()
	stbToken = blockchain.NewStableToken(endpoint).
		SetAllowance(allowance).
		SetBalance(balance).
		SetRegistry(reg).
		SetPause(pause).
		SetStable(stable)
	stbToken.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)

	// stable token set-up
	if err = stbToken.Start(); err != nil {
		return
	}

	// deploy fp token
	fpReg, err := deployContract(blockchain.FpRegistryBinary, endpoint)
	if err != nil {
		return
	}
	cdp, err := deployContract(blockchain.CdpManageBinary, endpoint)
	if err != nil {
		return
	}
	manage, err := deployContract(blockchain.ManageBinary, endpoint)
	if err != nil {
		return
	}
	proxy, err := deployContract(blockchain.ManageProxyBinary, endpoint)
	if err != nil {
		return
	}
	eap, err := deployContract(blockchain.EapStorageBinary, endpoint)
	if err != nil {
		return
	}
	riskLock, err := deployContract(blockchain.TokenRiskLockBinary, endpoint)
	if err != nil {
		return
	}

	// create fp token
	fpToken = blockchain.NewFpToken(endpoint).
		SetManagement(manage).
		SetManagementProxy(proxy).
		SetEapStorage(eap).
		SetRiskLock(riskLock).
		SetRegistry(fpReg).
		SetCdpManager(cdp).
		SetStableToken(stable)
	fpToken.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)

	// fp token set-up
	if err = fpToken.Start(); err != nil {
		return
	}

	// erc721 token set-up
	addr, err := deployContract(blockchain.Erc721Binary, endpoint)
	if err != nil {
		return
	}
	erc721Token = blockchain.NewErc721Token(endpoint)
	erc721Token.SetAddress(addr)
	erc721Token.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)

	err = erc721Token.Start()
	return
}

// GenerateAssetID generates an asset ID
func GenerateAssetID() string {
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

func deployContract(code, endpoint string, args ...[]byte) (string, error) {
	// deploy the contract
	contract := blockchain.NewContract(endpoint)
	h, err := contract.
		SetExecutor(blockchain.Producer).
		SetPrvKey(blockchain.ProducerPrivKey).
		Deploy(code, args...)
	if err != nil {
		return "", errors.Wrapf(err, "failed to deploy contract, txhash = %s", h)
	}

	receipt, err := contract.CheckCallResult(h)
	if err != nil {
		return h, errors.Wrapf(err, "check failed to deploy contract, txhash = %s", h)
	}
	return receipt.ContractAddress, nil
}
