// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	_stake2UpdateCmdUses = map[config.Language]string{
		config.English: "update NAME (ALIAS|OPERATOR_ADDRESS) (ALIAS|REWARD_ADDRESS)" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "update 名字 (别名|操作者地址) (别名|奖励地址)" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_stake2UpdateCmdShorts = map[config.Language]string{
		config.English: "Update candidate on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上更新候选人",
	}
)

var _stake2UpdateCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2UpdateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2UpdateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Update(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_stake2UpdateCmd)
}

func stake2Update(args []string) error {
	name := args[0]
	if err := validator.ValidateCandidateNameForStake2(name); err != nil {
		return output.NewError(output.ValidationError, "invalid candidate name", err)
	}

	operatorAddrStr, err := util.Address(args[1])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get operator address", err)
	}
	rewardAddrStr, err := util.Address(args[2])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get reward address", err)
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.CandidateUpdateBaseIntrinsicGas
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	s2u, err := action.NewCandidateUpdate(nonce, name, operatorAddrStr, rewardAddrStr, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a candidateUpdate instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2u).Build(),
		sender)
}
