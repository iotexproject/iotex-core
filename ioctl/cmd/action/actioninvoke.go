// Copyright (c) 2019 IoTeX Foundation
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
	invokeCmdShorts = map[config.Language]string{
		config.English: "Invoke smart contract on IoTeX blockchain\nWarning: 'ioctl action invoke' has been deprecated, use 'ioctl contract invoke' instead",
		config.Chinese: "在IoTeX区块链上调用智能合约\nWarning: 'ioctl action invoke' 已被废弃, 使用 'ioctl contract invoke' 代替",
	}
	invokeCmdUses = map[config.Language]string{
		config.English: "invoke (ALIAS|CONTRACT_ADDRESS) [AMOUNT_IOTX] -b BYTE_CODE [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "invoke (别名|联系人地址) [IOTX数量] -b 类型码 [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS" +
			"价格] [-P 密码] [-y]",
	}
)

// actionInvokeCmd represents the action invoke command
var actionInvokeCmd = &cobra.Command{
	Use:   config.TranslateInLang(invokeCmdUses, config.UILanguage),
	Short: config.TranslateInLang(invokeCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		var err error
		return output.NewError(output.RuntimeError, "'ioctl action invoke' has been deprecated, use 'ioctl contract invoke' instead", err)
	},
}
