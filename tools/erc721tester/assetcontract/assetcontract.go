// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package assetcontract

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/tools/erc721tester/blockchain"
	executiontester "github.com/iotexproject/iotex-core/tools/executiontester/blockchain"
)

const (
	// ChainIP is the ip address of iotex api endpoint
	chainIP = "localhost"
)
// StartContracts deploys and starts erc721 token smart contract
func StartContracts(cfg config.Config) (blockchain.Erc721Token,error) {
	endpoint := chainIP + ":" + strconv.Itoa(cfg.API.Port)

	addr, err := deployContract(blockchain.Erc721Binary, endpoint)
	if err != nil {
		return nil, err
	}
	erc721Token := blockchain.NewErc721Token(endpoint)
	erc721Token.SetAddress(addr)
	erc721Token.SetOwner(executiontester.Producer, executiontester.ProducerPrivKey)

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
	contract := executiontester.NewContract(endpoint)
	// extract this method
	setE:=contract.SetExecutor
	h, err := setE(executiontester.Producer).
		SetPrvKey(executiontester.ProducerPrivKey).
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
