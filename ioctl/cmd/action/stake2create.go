// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	_stake2CreateCmdUses = map[config.Language]string{
		config.English: "create AMOUNT_IOTX CANDIDATE_NAME STAKE_DURATION [DATA] [--auto-stake" +
			"] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GASP_RICE] [-P PASSWORD] [-y]",
		config.Chinese: "create IOTX数量 候选人名字 投票持续时间 [数据] [--auto-stake" +
			"] [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	_stake2CreateCmdShorts = map[config.Language]string{
		config.English: "Create bucket on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上创建投票",
	}
)

// _stake2CreateCmd represents the stake2 create command
var _stake2CreateCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2CreateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2CreateCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(3, 4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Create(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_stake2CreateCmd)
	_stake2CreateCmd.Flags().BoolVar(&_stake2AutoStake, "auto-stake", false,
		config.TranslateInLang(_stake2FlagAutoStakeUsages, config.UILanguage))
}

func stake2Create(args []string) error {
	var amount = args[0]
	amountInRau, err := util.StringToRau(amount, util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid amount", err)
	}
	amountStringInRau := amountInRau.String()

	var candidateName = args[1]
	if err := validator.ValidateCandidateNameForStake2(candidateName); err != nil {
		return output.NewError(output.ValidationError, "invalid candidate name", err)
	}
	stakeDuration, err := parseStakeDuration(args[2])
	if err != nil {
		return output.NewError(0, "", err)
	}
	duration := uint32(stakeDuration.Uint64())

	var data []byte
	if len(args) == 4 {
		data, err = hex.DecodeString(args[3])
		if err != nil {
			return output.NewError(output.ConvertError, "failed to decode data", err)
		}
	}

	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.CreateStakeBaseIntrinsicGas + action.CreateStakePayloadGas*uint64(len(data))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	s2c, err := action.NewCreateStake(nonce, candidateName, amountStringInRau, duration, _stake2AutoStake, data, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a createStake instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2c).Build(),
		sender)
}
