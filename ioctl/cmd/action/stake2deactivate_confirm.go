// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

var (
	_stake2DeactivateConfirmCmdUses = map[config.Language]string{
		config.English: "deactivate-confirm" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "deactivate-confirm" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}

	_stake2DeactivateConfirmCmdShorts = map[config.Language]string{
		config.English: "Confirm deactivation after the scheduled block has been reached",
		config.Chinese: "在退出调度区块到达后, 确认完成退出",
	}
)

var _stake2DeactivateConfirmCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2DeactivateConfirmCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2DeactivateConfirmCmdShorts, config.UILanguage),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(stake2DeactivateConfirm())
	},
}

func init() {
	RegisterWriteCommand(_stake2DeactivateConfirmCmd)
}

func stake2DeactivateConfirm() error {
	return sendStake2Deactivate(action.CandidateDeactivateOpConfirm)
}
