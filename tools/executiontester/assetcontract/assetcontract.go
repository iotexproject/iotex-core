package assetcontract

import (
	"time"
	"strconv"
	"strings"
	"math/rand"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/tools/executiontester/blockchain"
)

// StartContracts deploys and starts fp token smart contract and stable token smart contract
func StartContracts(chainEndPoint string) (blockchain.FpToken, blockchain.StableToken, error) {
	// deploy allowance sheet
	allowance, err := deployContract(blockchain.AllowanceSheetBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	// deploy balance sheet
	balance, err := deployContract(blockchain.BalanceSheetBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	// deploy registry
	reg, err := deployContract(blockchain.RegistryBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	// deploy global pause
	pause, err := deployContract(blockchain.GlobalPauseBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	// deploy stable token
	stable, err := deployContract(blockchain.StableTokenBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}

	// create stable token
	// TODO: query total supply and call stbToken.SetTotal()
	stbToken := blockchain.NewStableToken(chainEndPoint).
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
	fpReg, err := deployContract(blockchain.FpRegistryBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	cdp, err := deployContract(blockchain.CdpManageBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	manage, err := deployContract(blockchain.ManageBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	proxy, err := deployContract(blockchain.ManageProxyBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	eap, err := deployContract(blockchain.EapStorageBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}
	riskLock, err := deployContract(blockchain.TokenRiskLockBinary, chainEndPoint)
	if err != nil {
		return nil, nil, err
	}

	// create fp token
	fpToken := blockchain.NewFpToken(chainEndPoint).
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

func deployContract(code string, chainEndPoint string) (string, error) {
	// deploy the contract
	contract := blockchain.NewContract(chainEndPoint)
	h, err := contract.
		SetExecutor(blockchain.Producer).
		SetPubKey(blockchain.ProducerPubKey).
		SetPrvKey(blockchain.ProducerPrivKey).
		Deploy(code)
	if err != nil {
		return "", errors.Wrapf(err, "failed to deploy contract, txhash = %s", h)
	}

	time.Sleep(time.Second * 18)
	receipt, err := contract.CheckCallResult(h)
	if err != nil {
		return "", errors.Wrapf(err, "failed to deploy contract, txhash = %s", h)
	}
	return receipt.ContractAddress, nil
}
