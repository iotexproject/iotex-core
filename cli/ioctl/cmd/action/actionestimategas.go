// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// actionEstimateCmd represents the action estimate command
var actionEstimateCmd = &cobra.Command{
	Use: "estimate (ALIAS|CONTRACT_ADDRESS) [AMOUNT_IOTX]" +
		" [-s SIGNER] -b BYTE_CODE [-l GAS_LIMIT] [-p GAS_PRICE]",
	Short: "Estimate gas of action on IoTeX blockchain",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cmd.SilenceUsage = true
		var contract string
		if !strings.EqualFold(args[0], "") {
			contract, err = util.Address(args[0])
			if err != nil {
				return err
			}
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
		return estimateGas(contract, amount, bytecode)
	},
}

func init() {
	registerWriteCommand(actionEstimateCmd)
	bytecodeFlag.RegisterCommand(actionEstimateCmd)
	bytecodeFlag.MarkFlagRequired(actionEstimateCmd)
}
