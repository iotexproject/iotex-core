// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
)

// Xrc20BalanceofCmd represents balanceof function
var Xrc20BalanceofCmd = &cobra.Command{
	Use: "balanceof (ALIAS|OWNER_ADDRESS)" +
		" -c ALIAS|CONTRACT_ADDRESS ",
	Short: "Get account balance",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		addr, err := alias.Address(args[0])
		if err != nil {
			return err
		}
		xrc20OwnerAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		output, err := balanceOf(args)
		if err == nil {
			fmt.Println(output)
			result := new(big.Int)
			result.SetString(output, 16)
			fmt.Printf("Ouptut in decimal format: %d\n", result)
		}
		return err
	},
}

func toEthAddr(addr address.Address) common.Address {
	return common.BytesToAddress(addr.Bytes())
}

// read reads smart contract on IoTeX blockchain
func balanceOf(args []string) (string, error) {
	args[0] = xrc20ContractAddress
	signer = "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7"
	gasLimit = 50000
	var err error
	xrc20Bytes, err = xrc20ABI.Pack("balanceOf", toEthAddr(xrc20OwnerAddress))
	if err != nil {
		return "", err
	}
	return read(args)
}
