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
	deployBinCmdUses = map[config.Language]string{
		config.English: "bin BIN_PATH [ABI_PATH INIT_INPUT]",
		config.Chinese: "bin BIN BIN文件路径 [ABI文件路径 初始化参数]",
	}
	deployBinCmdShorts = map[config.Language]string{
		config.English: "deploy smart contract with bin on IoTeX blockchain",
		config.Chinese: "deploy 使用 bin 文件方式在 IoTex区块链上部署智能合约",
	}
)

// contractDeployBinCmd represents the contract deploy bin command
var contractDeployBinCmd = &cobra.Command{
	Use:   config.TranslateInLang(deployBinCmdUses, config.UILanguage),
	Short: config.TranslateInLang(deployBytecodeCmdShorts, config.UILanguage),
	Args:  util.CheckArgs(1, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contractDeployBin(args)
		return output.PrintError(err)
	},
}

func contractDeployBin(args []string) error {
	return nil
}
