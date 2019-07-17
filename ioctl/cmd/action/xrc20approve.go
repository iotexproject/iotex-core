// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// xrc20ApproveCmd could config target address limited amount
var xrc20ApproveCmd = &cobra.Command{
	Use: "approve (ALIAS|SPENDER_ADDRESS) (XRC20_AMOUNT)" +
		" -c ALIAS|CONTRACT_ADDRESS [-s SIGNER] [-l GAS_LIMIT] [-P PASSWORD] [-y]",
	Short: "Allow spender to withdraw from your account, multiple times, up to the amount",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		spender, err := alias.EtherAddress(args[0])
		if err != nil {
			return output.PrintError(output.AddressError, err.Error())
		}
		contract, err := xrc20Contract()
		if err != nil {
			return output.PrintError(output.AddressError, err.Error())
		}
		amount, err := parseAmount(contract, args[1])
		if err != nil {
			return output.PrintError(0, err.Error()) // TODO:undefined error
		}
		bytecode, err := xrc20ABI.Pack("approve", spender, amount)
		if err != nil {
			return output.PrintError(0, "cannot generate bytecode from given command"+err.Error()) // TODO: undefined error
		}
		return execute(contract.String(), big.NewInt(0), bytecode)
	},
}

func init() {
	registerWriteCommand(xrc20ApproveCmd)
}
