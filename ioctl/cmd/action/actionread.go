// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// actionReadCmd represents the action Read command
var actionReadCmd = &cobra.Command{
	Use:   "read (ALIAS|CONTRACT_ADDRESS) -b BYTE_CODE [-s SIGNER]",
	Short: "read smart contract on IoTeX blockchain",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := read(args[0])
		return output.PrintError(err)
	},
}

func init() {
	signerFlag.RegisterCommand(actionReadCmd)
	bytecodeFlag.RegisterCommand(actionReadCmd)
	bytecodeFlag.MarkFlagRequired(actionReadCmd)
}

func read(arg string) error {
	contract, err := alias.IOAddress(arg)
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	bytecode, err := decodeBytecode()
	if err != nil {
		return output.NewError(output.ConvertError, "invalid bytecode", err)
	}
	result, err := Read(contract, bytecode)
	if err != nil {
		return output.NewError(0, "failed to Read contract", err)
	}
	output.PrintResult(result)
	return err
}
