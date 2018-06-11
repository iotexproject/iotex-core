// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cli

import (
	"fmt"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
)

func (cli *CLI) getBalance(address string, config *config.Config) {
	if !iotxaddress.ValidateAddress(address) {
		logger.Fatal().Msg("ERROR: Address is not valid")
	}
	bc := blockchain.CreateBlockchain(config, nil)
	defer bc.Stop()

	balance := bc.BalanceOf(address)
	fmt.Printf("Balance of '%s': %d\n", address, balance)
}
