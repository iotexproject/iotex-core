// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	invokeFunctionCmdUses = map[config.Language]string{
		config.English: "function (CONTRACT_ADDRESS|ALIAS) FUNCTION_NAME [ABI_PATH INVOKE_INPUT]",
		config.Chinese: "function (合约地址|别名) 函数名 [ABI文件路径 调用初始化输入]",
	}
	invokeFunctionCmdShorts = map[config.Language]string{
		config.English: "invoke smart contract on IoTex blockchain with function name",
		config.Chinese: "invoke 通过 函数名方式 调用IoTex区块链上的智能合约",
	}
)

// contractInvokeFunctionCmd represents the contract invoke function command
var contractInvokeFunctionCmd = &cobra.Command{
	Use:   config.TranslateInLang(invokeFunctionCmdUses, config.UILanguage),
	Short: config.TranslateInLang(invokeFunctionCmdShorts, config.UILanguage),
	Args:  util.CheckArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractInvokeFunction(args)
		return output.PrintError(err)
	},
}

func contractInvokeFunction(args []string) error {
	return nil
}
