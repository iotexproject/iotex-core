package assetcontract

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/tools/erc721tester/blockchain"
)

const (
	// ChainIP is the ip address of iotex api endpoint
	chainIP = "localhost"
)

// StartContracts deploys and starts fp token smart contract and stable token smart contract
func StartContracts(cfg config.Config) (blockchain.Erc721Token,error) {
	endpoint := chainIP + ":" + strconv.Itoa(cfg.API.Port)

	addr, err := deployContract(blockchain.Erc721Binary, endpoint)
	if err != nil {
		return nil, err
	}
	erc721Token := blockchain.NewErc721Token(endpoint)
	erc721Token.SetAddress(addr)
	erc721Token.SetOwner(blockchain.Producer, blockchain.ProducerPrivKey)

	if err := erc721Token.Start(); err != nil {
		return nil, err
	}

	return erc721Token, nil
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
