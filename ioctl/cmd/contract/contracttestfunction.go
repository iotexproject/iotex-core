// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

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
	testFunctionCmdUses = map[config.Language]string{
		config.English: "function (CONTRACT_ADDRESS|ALIAS) ABI_PATH FUNCTION_NAME [AMOUNT_IOTX] " +
			"[--with-arguments INVOKE_INPUT]",
		config.Chinese: "function (合约地址|别名) ABI文件路径 函数名 [IOTX数量] [--with-arguments 调用输入]",
	}
	testFunctionCmdShorts = map[config.Language]string{
		config.English: "test smart contract on IoTeX blockchain with function name",
		config.Chinese: "调用函数测试IoTeX区块链上的智能合约",
	}
)

// contractTestFunctionCmd represents the contract test bytecode command
var contractTestFunctionCmd = &cobra.Command{
	Use:   config.TranslateInLang(testFunctionCmdUses, config.UILanguage),
	Short: config.TranslateInLang(testFunctionCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(3, 4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractTestFunction(args)
		return output.PrintError(err)
	},
}

func init() {
	withArgumentsFlag.RegisterCommand(contractTestFunctionCmd)
}

func contractTestFunction(args []string) error {
	addr, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	contract, err := address.FromString(addr)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert string into address", err)
	}

	abi, err := readAbiFile(args[1])
	if err != nil {
		return output.NewError(output.ReadFileError, "failed to read abi file "+args[1], err)
	}

	methodName := args[2]

	amount := big.NewInt(0)
	if len(args) == 4 {
		amount, err = util.StringToRau(args[3], util.IotxDecimalNum)
		if err != nil {
			return output.NewError(output.ConvertError, "invalid amount", err)
		}
	}

	bytecode, err := packArguments(abi, methodName, withArgumentsFlag.Value().(string))
	if err != nil {
		return output.NewError(output.ConvertError, "failed to pack given arguments", err)
	}

	rowResult, err := action.Read(contract, amount.String(), bytecode)
	if err != nil {
		return err
	}

	result, err := parseOutput(abi, methodName, rowResult)
	if err != nil {
		result = rowResult
	}

	output.PrintResult("return: " + result)
	return nil
}
