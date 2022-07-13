package assetcontract

import (
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/util/randutil"
	"github.com/iotexproject/iotex-core/tools/executiontester/blockchain"
)

const (
	// ChainIP is the ip address of iotex api endpoint
	chainIP = "localhost"
)

// ReturnedContract include all contract as return value
type ReturnedContract struct {
	FpToken          blockchain.FpToken
	StbToken         blockchain.StableToken
	Erc721Token      blockchain.Erc721Token
	ArrDeletePassing blockchain.ArrayDeletePassing
	ArrString        blockchain.ArrayString
}

// StartContracts deploys and starts fp token smart contract and stable token smart contract,erc721 token smart contract
func StartContracts(cfg config.Config) (ret ReturnedContract, err error) {
	endpoint := chainIP + ":" + strconv.Itoa(cfg.API.GRPCPort)

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
	ret.StbToken = blockchain.NewStableToken(endpoint).
		SetAllowance(allowance).
		SetBalance(balance).
		SetRegistry(reg).
		SetPause(pause).
		SetStable(stable)
	ret.StbToken.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)

	// stable token set-up
	if err = ret.StbToken.Start(); err != nil {
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
	ret.FpToken = blockchain.NewFpToken(endpoint).
		SetManagement(manage).
		SetManagementProxy(proxy).
		SetEapStorage(eap).
		SetRiskLock(riskLock).
		SetRegistry(fpReg).
		SetCdpManager(cdp).
		SetStableToken(stable)
	ret.FpToken.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)

	// fp token set-up
	if err = ret.FpToken.Start(); err != nil {
		return
	}

	// erc721 token set-up
	addr, err := deployContract(blockchain.Erc721Binary, endpoint)
	if err != nil {
		return
	}
	ret.Erc721Token = blockchain.NewErc721Token(endpoint)
	ret.Erc721Token.SetAddress(addr)
	ret.Erc721Token.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)

	if err = ret.Erc721Token.Start(); err != nil {
		return
	}
	// array-delete-passing.sol set-up
	addr, err = deployContract(blockchain.ArrayDeletePassingBin, endpoint)
	if err != nil {
		return
	}
	ret.ArrDeletePassing = blockchain.NewArrayDelete(endpoint)
	ret.ArrDeletePassing.SetAddress(addr)
	ret.ArrDeletePassing.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)
	if err = ret.ArrDeletePassing.Start(); err != nil {
		return
	}
	// array-of-strings.sol set-up
	addr, err = deployContract(blockchain.ArrayStringBin, endpoint)
	if err != nil {
		return
	}
	ret.ArrString = blockchain.NewArrayString(endpoint)
	ret.ArrString.SetAddress(addr)
	ret.ArrString.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)
	err = ret.ArrString.Start()
	return
}

// GenerateAssetID generates an asset ID
func GenerateAssetID() string {
	for {
		id := strconv.Itoa(randutil.Int())
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
