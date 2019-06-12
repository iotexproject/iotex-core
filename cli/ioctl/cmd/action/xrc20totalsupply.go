// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// Xrc20TotalsupplyCmd represents total supply of the contract
var Xrc20TotalsupplyCmd = &cobra.Command{
	Use: "totalsupply" +
		" -c CONTRACT_ADDRESS",
	Short: "Get total supply",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := totalSupply(args)
		if err == nil {
			fmt.Println(output)
			result, _ := strconv.ParseUint(output, 16, 64)
			fmt.Println("Output in decimal format:")
			fmt.Println(uint64(result))
		}
		return err
	},
}

// read reads smart contract on IoTeX blockchain
func totalSupply(args []string) (string, error) {
	argument := make([]string, 1)
	argument[0] = contractAddress
	signer = "ALIAS"
	gasLimit = 50000
	var err error
	bytes, err = abiResult.Pack("totalSupply")
	if err != nil {
		return "", err
	}
	return read(argument)
}
