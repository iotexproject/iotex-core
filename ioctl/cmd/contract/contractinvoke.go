// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_invokeCmdShorts = map[config.Language]string{
		config.English: "Invoke smart contract on IoTeX blockchain",
		config.Chinese: "调用IoTeX区块链上的智能合约",
	}
)

// _contractInvokeCmd represents the contract invoke command
var _contractInvokeCmd = &cobra.Command{
	Use:   "invoke",
	Short: config.TranslateInLang(_invokeCmdShorts, config.UILanguage),
}

func init() {
	_contractInvokeCmd.AddCommand(_contractInvokeFunctionCmd)
	_contractInvokeCmd.AddCommand(_contractInvokeBytecodeCmd)
	action.RegisterWriteCommand(_contractInvokeFunctionCmd)
	action.RegisterWriteCommand(_contractInvokeBytecodeCmd)

}
