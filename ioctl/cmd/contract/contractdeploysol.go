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
)

// Multi-language support
var (
	deploySolCmdUses = map[config.Language]string{
		config.English: "sol CODE_PATH CONTRACT_NAME [INIT_INPUT]",
		config.Chinese: "sol 源代码文件路径 合约名 [初始化参数]",
	}
	deploySolCmdShorts = map[config.Language]string{
		config.English: "deploy smart contract with sol on IoTeX blockchain",
		config.Chinese: "deploy 使用 sol 方式在 IoTex区块链上部署智能合约",
	}
)

// contractDeploySolCmd represents the contract deploy sol command
var contractDeploySolCmd = &cobra.Command{
	Use:   config.TranslateInLang(deploySolCmdUses, config.UILanguage),
	Short: config.TranslateInLang(deployBytecodeCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractDeploySol(args)
		return output.PrintError(err)
	},
}

func contractDeploySol(args []string) error {
	return nil
}
