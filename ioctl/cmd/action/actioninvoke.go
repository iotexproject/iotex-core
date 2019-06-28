// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// actionInvokeCmd represents the action invoke command
var actionInvokeCmd = &cobra.Command{
	Use: "invoke (ALIAS|CONTRACT_ADDRESS) [AMOUNT_IOTX]" +
		" [-s SIGNER] -b BYTE_CODE [-l GAS_LIMIT] [-p GAS_PRICE]",
	Short: "Invoke smart contract on IoTeX blockchain",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		contract, err := util.Address(args[0])
		if err != nil {
			return err
		}
		amount := big.NewInt(0)
		if len(args) == 2 {
			amount, err = util.StringToRau(args[1], util.IotxDecimalNum)
			if err != nil {
				return err
			}
		}
		bytecode, err := decodeBytecode()
		if err != nil {
			return err
		}
		return execute(contract, amount, bytecode)
	},
}

func init() {
	registerWriteCommand(actionInvokeCmd)
	bytecodeFlag.RegisterCommand(actionInvokeCmd)
	bytecodeFlag.MarkFlagRequired(actionInvokeCmd)
}
