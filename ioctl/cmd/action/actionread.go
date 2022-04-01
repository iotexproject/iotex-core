// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_readCmdShorts = map[config.Language]string{
		config.English: "read smart contract on IoTeX blockchain",
		config.Chinese: "读取IoTeX区块链上的智能合约",
	}
	_readCmdUses = map[config.Language]string{
		config.English: "read (ALIAS|CONTRACT_ADDRESS) -b BYTE_CODE [-s SIGNER]",
		config.Chinese: "reads (别名|联系人地址) -b 类型码 [-s 签署人]",
	}
)

// _actionReadCmd represents the action Read command
var _actionReadCmd = &cobra.Command{
	Use:   config.TranslateInLang(_readCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_readCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := read(args[0])
		return output.PrintError(err)
	},
}

func init() {
	_signerFlag.RegisterCommand(_actionReadCmd)
	_bytecodeFlag.RegisterCommand(_actionReadCmd)
	_bytecodeFlag.MarkFlagRequired(_actionReadCmd)
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
	result, err := Read(contract, "0", bytecode)
	if err != nil {
		return output.NewError(0, "failed to Read contract", err)
	}
	output.PrintResult(result)
	return err
}
