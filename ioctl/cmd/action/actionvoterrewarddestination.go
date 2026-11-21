// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
)

// Multi-language support
var (
	_voterRewardDestinationCmdUses = map[config.Language]string{
		config.English: "voterrewarddestination [ALIAS|RECIPIENT_ADDRESS] [--reset]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "voterrewarddestination [别名|收款地址] [--reset]" +
			" [-s 签署人] [-n NONCE] [-l GAS 限制] [-p GAS 价格] [-P 密码] [-y]",
	}

	_voterRewardDestinationCmdShorts = map[config.Language]string{
		config.English: "Route the signer's IIP-59 voter rewards to another address",
		config.Chinese: "将签署人的 IIP-59 投票人奖励转到其它地址",
	}

	_voterRewardDestinationCmdLong = map[config.Language]string{
		config.English: "Set where the signer's IIP-59 voter rewards are credited.\n\n" +
			"By default a voter's rewards are credited to the voting address itself. This\n" +
			"action overrides that, which is useful when voting from a key you would rather\n" +
			"not also use for withdrawals.\n\n" +
			"--reset removes the override and sends rewards back to the signer. Naming the\n" +
			"signer's own address does the same thing.\n\n" +
			"Only affects the on-chain path, so it does nothing for a delegate that has not\n" +
			"opted in. Use 'ioctl node reward destination VOTER' to read the current value.",
		config.Chinese: "设置签署人的 IIP-59 投票人奖励打到哪个地址。\n\n" +
			"默认情况下投票人奖励打给投票地址本身。此操作可以覆盖该行为，\n" +
			"适用于不希望用投票私钥同时管理提现的场景。\n\n" +
			"--reset 取消覆盖、奖励打回签署人自己；填签署人自己的地址效果相同。\n\n" +
			"只影响链上发放路径，因此对尚未开启 IIP-59 的候选人无效。\n" +
			"可用 'ioctl node reward destination VOTER' 查询当前值。",
	}

	_flagVoterRewardDestinationReset = map[config.Language]string{
		config.English: "clear the override and credit rewards to the signer",
		config.Chinese: "清除覆盖，奖励打回签署人自己",
	}
)

var _voterRewardDestinationReset = flag.BoolVarP("reset", "", false,
	config.TranslateInLang(_flagVoterRewardDestinationReset, config.UILanguage))

var (
	// _actionVoterRewardDestinationCmd represents the action voterrewarddestination command
	_actionVoterRewardDestinationCmd = &cobra.Command{
		Use:   config.TranslateInLang(_voterRewardDestinationCmdUses, config.UILanguage),
		Short: config.TranslateInLang(_voterRewardDestinationCmdShorts, config.UILanguage),
		Long:  config.TranslateInLang(_voterRewardDestinationCmdLong, config.UILanguage),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			err := setVoterRewardDestination(args)
			return output.PrintError(err)
		},
	}
)

func init() {
	_voterRewardDestinationReset.RegisterCommand(_actionVoterRewardDestinationCmd)
	RegisterWriteCommand(_actionVoterRewardDestinationCmd)
}

func setVoterRewardDestination(args []string) error {
	reset := _voterRewardDestinationReset.Value().(bool)
	// Requiring one or the other keeps a bare invocation from silently
	// clearing an override the caller forgot they had set.
	switch {
	case reset && len(args) > 0:
		return output.NewError(output.InputError,
			"--reset takes no recipient; pass either an address or --reset, not both", nil)
	case !reset && len(args) == 0:
		return output.NewError(output.InputError,
			"specify a recipient address, or --reset to credit rewards to the signer", nil)
	}

	var recipient []byte
	if !reset {
		addrStr, err := util.Address(args[0])
		if err != nil {
			return output.NewError(output.AddressError, "failed to resolve recipient address", err)
		}
		addr, err := address.FromString(addrStr)
		if err != nil {
			return output.NewError(output.AddressError, "invalid recipient address", err)
		}
		recipient = addr.Bytes()
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.SetVoterRewardDestinationBaseIntrinsicGas
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
			SetAction(action.NewSetVoterRewardDestination(recipient)).Build(),
		sender)
}
