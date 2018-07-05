// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
)

func (cli *CLI) send(from, to string, amount uint64, config *config.Config) {
	ctx := context.Background()
	if !iotxaddress.ValidateAddress(from) {
		logger.Fatal().Msg("ERROR: Sender address is not valid")
	}
	if !iotxaddress.ValidateAddress(to) {
		logger.Fatal().Msg("ERROR: Recipient address is not valid")
	}

	bc := blockchain.NewBlockchain(config, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
	defer bc.Stop(ctx)

	//tx := blockchain.NewUTXOTransaction(from, to, amount, bc)
	//bc.MineBlock([]*blockchain.Tx{tx})
	fmt.Println("Success!")
}
