// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	deployBytecodeCmdUses = map[config.Language]string{
		config.English: "bytecode BYTECODE [ABI_PATH INIT_INPUT]",
		config.Chinese: "bytecode BYTECODE [ABI文件路径 初始化输入]",
	}
	deployBytecodeCmdShorts = map[config.Language]string{
		config.English: "deploy smart contract with bytecode on IoTeX blockchain",
		config.Chinese: "deploy 使用 bytecode 文件方式在 IoTex区块链上部署智能合约",
	}
)

// contractDeployBytecodeCmd represents the contract deploy bytecode command
var contractDeployBytecodeCmd = &cobra.Command{
	Use:   config.TranslateInLang(deployBytecodeCmdUses, config.UILanguage),
	Short: config.TranslateInLang(deployBytecodeCmdShorts, config.UILanguage),
	Args:  util.CheckArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractDeployBytecode(args)
		return output.PrintError(err)
	},
}

func contractDeployBytecode(args []string) error {
	bytecode, err := decodeBytecode(args[0])
	if err != nil {
		return output.NewError(output.ConvertError, "failed to decode bytecode", err)
	}

	if len(args) == 3 {
		abi, err := readAbiFile(args[1])
		if err != nil {
			return err
		}
		// Constructor's method name is "" (empty string)
		packedArg, err := packArguments(abi, "", args[2])
		if err != nil {
			return output.NewError(output.ConvertError, "failed to pack given arguments", err)
		}

		bytecode = append(bytecode, packedArg...)
	}

	amount := big.NewInt(0)
	return action.Execute("", amount, bytecode)
}
