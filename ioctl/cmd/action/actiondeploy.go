// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_deployCmdShorts = map[config.Language]string{
		config.English: "Deploy smart contract on IoTeX blockchain \nWarning: 'ioctl action deploy' has been deprecated, use 'ioctl contract deploy' instead",
		config.Chinese: "在IoTeX区块链上部署智能合约 \nWarning: 'ioctl action deploy' 已被废弃, 使用 'ioctl contract deploy' 代替",
	}
	_deployCmdUses = map[config.Language]string{
		config.English: "deploy [AMOUNT_IOTX] [-s SIGNER] -b BYTE_CODE [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "deploy [IOTX数量] [-s 签署人] -b 类型码 [-n NONCE] [-l GAS限制] [-p GAS价格] [-P" +
			" 密码] [-y]",
	}
)

// _actionDeployCmd represents the action deploy command
// Deprecated: notify users to use the new ioctl contract command
// TODO: this command will be deprecated soon
var _actionDeployCmd = &cobra.Command{
	Use:                config.TranslateInLang(_deployCmdUses, config.UILanguage),
	Short:              config.TranslateInLang(_deployCmdShorts, config.UILanguage),
	Hidden:             true,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		var err error
		return output.NewError(output.RuntimeError, "'ioctl action deploy' has been deprecated, use 'ioctl contract deploy' instead", err)
	},
}
