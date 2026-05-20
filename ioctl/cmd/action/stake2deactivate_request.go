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
	_stake2DeactivateRequestCmdUses = map[config.Language]string{
		config.English: "deactivate-request" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "deactivate-request" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}

	_stake2DeactivateRequestCmdShorts = map[config.Language]string{
		config.English: "Request to deactivate the signer's candidate (start the exit queue)",
		config.Chinese: "申请退出: 将签署人对应的候选人加入退出队列",
	}
)

var _stake2DeactivateRequestCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2DeactivateRequestCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2DeactivateRequestCmdShorts, config.UILanguage),
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return output.PrintError(stake2DeactivateRequest())
	},
}

func init() {
	RegisterWriteCommand(_stake2DeactivateRequestCmd)
}

func stake2DeactivateRequest() error {
	return sendStake2Deactivate(action.CandidateDeactivateOpRequest)
}
