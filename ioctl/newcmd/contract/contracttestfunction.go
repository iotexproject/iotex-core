// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
	"github.com/iotexproject/iotex-core/v2/ioctl/newcmd/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
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
)

// NewContractTestFunctionCmd represents the contract test function cmd
func NewContractTestFunctionCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_testFunctionCmdUses)
	short, _ := client.SelectTranslation(_testFunctionCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return contractTestFunction(client, cmd, args)
		},
	}
	action.RegisterWriteCommand(client, cmd)
	flag.WithArgumentsFlag.RegisterCommand(cmd)
	return cmd
}

func contractTestFunction(client ioctl.Client, cmd *cobra.Command, args []string) error {
	addr, err := client.Address(args[0])
	if err != nil {
		return errors.WithMessage(err, "failed to get contract address")
	}

	contract, err := address.FromString(addr)
	if err != nil {
		return errors.WithMessage(err, "failed to convert string into address")
	}

	abi, err := readAbiFile(args[1])
	if err != nil {
		return errors.WithMessage(err, "failed to read abi file "+args[1])
	}

	methodName := args[2]

	amount := big.NewInt(0)
	if len(args) == 4 {
		amount, err = util.StringToRau(args[3], util.IotxDecimalNum)
		if err != nil {
			return errors.Wrap(err, "invalid amount")
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

	cmd.Println("return: " + result)
	return nil
}
