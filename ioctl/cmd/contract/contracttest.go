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
	testCmdUses = map[config.Language]string{
		config.English: "test",
		config.Chinese: "test",
	}
	testCmdShorts = map[config.Language]string{
		config.English: "Test smart contract of IoTeX blockchain",
		config.Chinese: "测试IoTeX区块链部署智能合约",
	}
)

// contractTesCmd represents the contract test command
var contractTestCmd = &cobra.Command{
	Use:   config.TranslateInLang(testCmdUses, config.UILanguage),
	Short: config.TranslateInLang(testCmdShorts, config.UILanguage),
}

func init() {
	contractTestCmd.AddCommand(contractTestBytecodeCmd)
	contractTestCmd.AddCommand(contractTestFunctionCmd)
	action.RegisterWriteCommand(contractTestBytecodeCmd)
	action.RegisterWriteCommand(contractTestFunctionCmd)
}
