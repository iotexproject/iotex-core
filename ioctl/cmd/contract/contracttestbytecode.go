// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_testBytecodeCmdUses = map[config.Language]string{
		config.English: "bytecode (CONTRACT_ADDRESS|ALIAS) PACKED_ARGUMENTS [AMOUNT_IOTX]",
		config.Chinese: "bytecode (合约地址|别名) 已打包参数 [IOTX数量]",
	}
	_testBytecodeCmdShorts = map[config.Language]string{
		config.English: "test smart contract on IoTeX blockchain with packed arguments",
		config.Chinese: "传入bytecode测试IoTeX区块链上的智能合约",
	}
)

// _contractTestBytecodeCmd represents the contract test bytecode command
var _contractTestBytecodeCmd = &cobra.Command{
	Use:   config.TranslateInLang(_testBytecodeCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_testBytecodeCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractTestBytecode(args)
		return output.PrintError(err)
	},
}

func contractTestBytecode(args []string) error {
	addr, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	contract, err := address.FromString(addr)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert string into address", err)
	}

	bytecode, err := decodeBytecode(args[1])
	if err != nil {
		return output.NewError(output.ConvertError, "invalid bytecode", err)
	}

	amount := big.NewInt(0)
	if len(args) == 3 {
		amount, err = util.StringToRau(args[2], util.IotxDecimalNum)
		if err != nil {
			return output.NewError(output.ConvertError, "invalid amount", err)
		}
	}

	result, err := action.Read(contract, amount.String(), bytecode)
	if err != nil {
		return err
	}

	output.PrintResult("return: " + result)
	return nil
}
