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
	_deployCmdShorts = map[config.Language]string{
		config.English: "Deploy smart contract of IoTeX blockchain",
		config.Chinese: "在IoTeX区块链部署智能合约",
	}
)

// NewContractDeployCmd represents the contract deploy command
func NewContractDeployCmd(client ioctl.Client) *cobra.Command {
	short, _ := client.SelectTranslation(_deployCmdShorts)
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: short,
	}

	// TODO add sub commands
	// cmd.AddCommand(NewcontractDeployBytecodeCmd)
	// cmd.AddCommand(NewContractDeployBinCmd)
	// cmd.AddCommand(NewContractDeploySolCmd)
	// action.RegisterWriteCommand(NewContractDeployBytecodeCmd)
	// action.RegisterWriteCommand(NewContractDeployBinCmd)
	// action.RegisterWriteCommand(NewContractDeploySolCmd)
	return cmd
}
