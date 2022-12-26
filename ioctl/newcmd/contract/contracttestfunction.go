// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Multi-language support
var (
	_testFunctionCmdUses = map[config.Language]string{
		config.English: "function (CONTRACT_ADDRESS|ALIAS) ABI_PATH FUNCTION_NAME [AMOUNT_IOTX] " +
			"[--with-arguments INVOKE_INPUT]",
		config.Chinese: "function (合约地址|别名) ABI文件路径 函数名 [IOTX数量] [--with-arguments 调用输入]",
	}
	_testFunctionCmdShorts = map[config.Language]string{
		config.English: "test smart contract on IoTeX blockchain with function name",
		config.Chinese: "调用函数测试IoTeX区块链上的智能合约",
	}
	_failToGetAddress = map[config.Language]string{
		config.English: "failed to get contract address",
		config.Chinese: "获取合约地址失败",
	}
	_failToConvertStringIntoAddress = map[config.Language]string{
		config.English: "failed to convert string into address",
		config.Chinese: "转换字符串到地址失败",
	}
	_failToReadABIFile = map[config.Language]string{
		config.English: "failed to read abi file",
		config.Chinese: "读取 ABI 文件失败",
	}
	_InvalidAmount = map[config.Language]string{
		config.English: "invalid amount",
		config.Chinese: "无效的数字",
	}
)

// NewContractTestFunctionCmd represents the contract test function cmd
func NewContractTestFunctionCmd(client ioctl.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   translate(client, _testFunctionCmdUses),
		Short: translate(client, _testFunctionCmdShorts),
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return contractTestFunction(client, cmd, args)
		},
	}
	return cmd
}

func contractTestFunction(client ioctl.Client, cmd *cobra.Command, args []string) error {
	addr, err := client.Address(args[0])
	if err != nil {
		return errors.WithMessage(err, translate(client, _failToGetAddress))
	}

	contract, err := address.FromString(addr)
	if err != nil {
		return errors.WithMessage(err, translate(client, _failToConvertStringIntoAddress))
	}

	abi, err := readAbiFile(args[1])
	if err != nil {
		return errors.WithMessage(err, translate(client, _failToReadABIFile)+" "+args[1])
	}

	methodName := args[2]

	amount := big.NewInt(0)
	if len(args) == 4 {
		amount, err = util.StringToRau(args[3], util.IotxDecimalNum)
		if err != nil {
			return errors.WithMessage(err, translate(client, _InvalidAmount))
		}
	}

	bytecode, err := packArguments(abi, methodName, flag.WithArgumentsFlag.Value().(string))
	if err != nil {
		return errors.WithMessage(err, "failed to pack given arguments")
	}

	_, signer, _, _, gasLimit, _, err := action.GetWriteCommandFlag(cmd)
	if err != nil {
		return errors.WithMessage(err, "failed to get command flag")
	}

	rowResult, err := action.Read(client, contract, amount.String(), bytecode, signer, gasLimit)
	if err != nil {
		return err
	}

	result, err := parseOutput(abi, methodName, rowResult)
	if err != nil {
		result = rowResult
	}

	fmt.Println("return: " + result)
	return nil
}
