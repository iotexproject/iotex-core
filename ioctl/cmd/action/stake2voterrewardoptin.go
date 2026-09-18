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

// Multi-language support
var (
	_stake2VoterRewardOptInCmdUses = map[config.Language]string{
		config.English: "voterrewardoptin" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "voterrewardoptin" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}

	_stake2VoterRewardOptInCmdShorts = map[config.Language]string{
		config.English: "Opt the candidate into on-chain voter reward distribution (IIP-59)",
		config.Chinese: "为候选人开启链上投票人奖励发放 (IIP-59)",
	}

	_stake2VoterRewardOptInCmdLong = map[config.Language]string{
		config.English: "Opt the signer's candidate into IIP-59 on-chain voter reward distribution.\n\n" +
			"Until a candidate opts in, its voter rewards keep flowing through the off-chain\n" +
			"path and the protocol distributes nothing on its behalf. The flag is one-way:\n" +
			"there is no action to opt back out.\n\n" +
			"Sign with the candidate's owner address. Two further things must be true before\n" +
			"voters are actually paid:\n" +
			"  - the fork that activates IIP-59 must be live on this network;\n" +
			"  - the candidate must have blockRewardPortion / epochRewardPortion set in the\n" +
			"    DelegateProfile contract, otherwise it is snapshotted at 100% commission and\n" +
			"    its voters receive nothing.\n\n" +
			"Use 'ioctl node reward snapshot DELEGATE' to check the frozen commission rates.",
		config.Chinese: "为签署人的候选人开启 IIP-59 链上投票人奖励发放。\n\n" +
			"在候选人开启之前，投票人奖励仍走链下路径，协议不代其发放任何奖励。\n" +
			"该标记是单向的：没有关闭的操作。\n\n" +
			"请使用候选人的 owner 地址签名。投票人真正拿到奖励还需要满足：\n" +
			"  - 本网络已激活 IIP-59 所在的硬分叉；\n" +
			"  - 候选人已在 DelegateProfile 合约中设置 blockRewardPortion / epochRewardPortion，\n" +
			"    否则冻结快照时按 100% 佣金处理，投票人拿不到任何奖励。\n\n" +
			"可用 'ioctl node reward snapshot DELEGATE' 查看冻结的佣金比例。",
	}
)

var (
	// _stake2VoterRewardOptInCmd represents the stake2 voterrewardoptin command
	_stake2VoterRewardOptInCmd = &cobra.Command{
		Use:   config.TranslateInLang(_stake2VoterRewardOptInCmdUses, config.UILanguage),
		Short: config.TranslateInLang(_stake2VoterRewardOptInCmdShorts, config.UILanguage),
		Long:  config.TranslateInLang(_stake2VoterRewardOptInCmdLong, config.UILanguage),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := stake2VoterRewardOptIn()
			return output.PrintError(err)
		},
	}
)

func init() {
	RegisterWriteCommand(_stake2VoterRewardOptInCmd)
}

func stake2VoterRewardOptIn() error {
	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.SetVoterRewardOptInBaseIntrinsicGas
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(action.NewSetVoterRewardOptIn()).Build(),
		sender)
}
