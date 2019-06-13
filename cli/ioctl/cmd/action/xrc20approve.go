// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
)

// Xrc20ApproveCmd could config target address limited amount
var Xrc20ApproveCmd = &cobra.Command{
	Use: "approve (ALIAS|SPENDER_ADDRESS) (AMOUNT)" +
		" -c ALIAS|CONTRACT_ADDRESS -s SIGNER -l GAS_LIMIT ",
	Short: "Allow spender to withdraw from your account, multiple times, up to the amount",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		addr, err := alias.Address(args[0])
		if err != nil {
			return err
		}
		xrc20SpenderAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		xrc20TransferAmount.SetString(args[1], 10)
		output, err := approve(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// read reads smart contract on IoTeX blockchain
func approve(args []string) (string, error) {
	var err error
	args[0] = xrc20ContractAddress
	args[1] = "0"
	xrc20Bytes, err = xrc20ABI.Pack("approve", toEthAddr(xrc20SpenderAddress), &xrc20TransferAmount)
	if err != nil {
		return "", err
	}
	return invoke(args)
}
