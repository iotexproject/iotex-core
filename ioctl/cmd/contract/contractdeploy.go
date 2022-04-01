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
	_deployCmdUses = map[config.Language]string{
		config.English: "deploy",
		config.Chinese: "deploy",
	}
	_deployCmdShorts = map[config.Language]string{
		config.English: "Deploy smart contract of IoTeX blockchain",
		config.Chinese: "在IoTeX区块链部署智能合约",
	}
)

// _contractDeployCmd represents the contract deploy command
var _contractDeployCmd = &cobra.Command{
	Use:   config.TranslateInLang(_deployCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_deployCmdShorts, config.UILanguage),
}

func init() {
	_contractDeployCmd.AddCommand(_contractDeployBytecodeCmd)
	_contractDeployCmd.AddCommand(_contractDeployBinCmd)
	_contractDeployCmd.AddCommand(_contractDeploySolCmd)
	action.RegisterWriteCommand(_contractDeployBytecodeCmd)
	action.RegisterWriteCommand(_contractDeployBinCmd)
	action.RegisterWriteCommand(_contractDeploySolCmd)
}
