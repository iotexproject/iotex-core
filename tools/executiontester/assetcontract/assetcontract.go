package assetcontract

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/tools/executiontester/blockchain"
)

// StartContracts deploys and starts fp token smart contract and stable token smart contract
func StartContracts(cfg config.Config) (blockchain.FpToken, blockchain.StableToken, error) {
	// deploy allowance sheet
	allowance, err := deployContract(blockchain.AllowanceSheetBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	// deploy balance sheet
	balance, err := deployContract(blockchain.BalanceSheetBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	// deploy registry
	reg, err := deployContract(blockchain.RegistryBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	// deploy global pause
	pause, err := deployContract(blockchain.GlobalPauseBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	// deploy stable token
	stable, err := deployContract(blockchain.StableTokenBinary, cfg)
	if err != nil {
		return nil, nil, err
	}

	// create stable token
	// TODO: query total supply and call stbToken.SetTotal()
	stbToken := blockchain.NewStableToken(cfg).
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
	fpReg, err := deployContract(blockchain.FpRegistryBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	cdp, err := deployContract(blockchain.CdpManageBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	manage, err := deployContract(blockchain.ManageBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	proxy, err := deployContract(blockchain.ManageProxyBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	eap, err := deployContract(blockchain.EapStorageBinary, cfg)
	if err != nil {
		return nil, nil, err
	}
	riskLock, err := deployContract(blockchain.TokenRiskLockBinary, cfg)
	if err != nil {
		return nil, nil, err
	}

	// create fp token
	fpToken := blockchain.NewFpToken(cfg).
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

func deployContract(code string, cfg config.Config) (string, error) {
	// deploy the contract
	contract := blockchain.NewContract(cfg)
	h, err := contract.
		SetExecutor(blockchain.Producer).
		SetPubKey(blockchain.ProducerPubKey).
		SetPrvKey(blockchain.ProducerPrivKey).
		Deploy(code)
	if err != nil {
		return "", errors.Wrapf(err, "failed to deploy contract, txhash = %s", h)
	}

	var receipt *iotextypes.Receipt
	var innerErr error
	err = testutil.WaitUntil(100*time.Millisecond, cfg.Genesis.BlockInterval*3, func() (bool, error) {
		if receipt, innerErr = contract.CheckCallResult(h); receipt != nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return "", errors.Wrapf(innerErr, "failed to deploy contract, txhash = %s", h)
	}
	return receipt.ContractAddress, nil
}
