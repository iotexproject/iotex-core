// Copyright (c) 2020 IoTeX Foundation
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
	invokeCmdUses = map[config.Language]string{
		config.English: "invoke",
		config.Chinese: "invoke",
	}
	invokeCmdShorts = map[config.Language]string{
		config.English: "Invoke smart contract on IoTeX blockchain",
		config.Chinese: "调用IoTeX区块链上的智能合约",
	}
)

// contractInvokeCmd represents the contract invoke command
var contractInvokeCmd = &cobra.Command{
	Use:   config.TranslateInLang(invokeCmdUses, config.UILanguage),
	Short: config.TranslateInLang(invokeCmdShorts, config.UILanguage),
}

func init() {
	contractInvokeCmd.AddCommand(contractInvokeFunctionCmd)
	contractInvokeCmd.AddCommand(contractInvokeBytecodeCmd)
	action.RegisterWriteCommand(contractInvokeFunctionCmd)
	action.RegisterWriteCommand(contractInvokeBytecodeCmd)

}
