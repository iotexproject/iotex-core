// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement,
// merchantability or fitness for purpose and, to the extent permitted by law,
// all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/ioctl"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
)

// Multi-language support
var (
	_invokeCmdShorts = map[config.Language]string{
		config.English: "Invoke smart contract on IoTeX blockchain",
		config.Chinese: "调用IoTeX区块链上的智能合约",
	}
)

// NewContractInvokeCmd represents the contract invoke command
func NewContractInvokeCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_invokeCmdShorts)

	cmd := &cobra.Command{
		Use:   "invoke",
		Short: short,
	}

	// TODO add sub commands
	// cmd.AddCommand(NewContractInvokeFunctionCmd)
	// cmd.AddCommand(NewContractInvokeBytecodeCmd)
	// action.RegisterWriteCommand(NewContractInvokeFunctionCmd)
	// action.RegisterWriteCommand(NewContractInvokeBytecodeCmd)
	return cmd
}
