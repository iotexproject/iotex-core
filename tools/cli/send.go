// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cli

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

func (cli *CLI) send(from, to string, amount uint64, config *config.Config) {
	if !iotxaddress.ValidateAddress(from) {
		glog.Fatal("ERROR: Sender address is not valid")
	}
	if !iotxaddress.ValidateAddress(to) {
		glog.Fatal("ERROR: Recipient address is not valid")
	}

	bc := blockchain.CreateBlockchain(from, config, blockchain.Gen)
	defer bc.Close()

	//tx := blockchain.NewUTXOTransaction(from, to, amount, bc)
	//bc.MineBlock([]*blockchain.Tx{tx})
	fmt.Println("Success!")
}
