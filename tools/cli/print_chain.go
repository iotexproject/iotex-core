// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cli

import (
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
)

func (cli *CLI) printChain(config *config.Config) {
	cli.bc = blockchain.CreateBlockchain(config, blockchain.Gen, nil)
	defer cli.bc.Stop()

	/*
		it := cli.bc.Iterator()
		for {
			block := it.Next()
			for i, tx := range block.Tranxs {
				fmt.Printf("Transaction %d: %x\n", i, tx.Hash())
			}
			fmt.Printf("Hash: %x\n", block.HashBlock())
			fmt.Println()

			if bytes.Compare(it.CurrHash, blockchain.ZeroHash32B[:]) == 0 {
				break
			}
		}
	*/
}
