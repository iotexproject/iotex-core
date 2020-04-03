// Copyright (c) 2020 IoTeX Foundation
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
	stake2CreateCmdUses = map[config.Language]string{
		config.English: "create AMOUNT_IOTX CANDIDATE_NAME STAKE_DURATION [DATA] [--auto-restake]",
		config.Chinese: "create IOTX数量 候选人姓名 权益持续时间 [数据] [--auto-restake]",
	}

	stake2CreateCmdShorts = map[config.Language]string{
		config.English: "create stake on IoTeX blockchain",
		config.Chinese: "在区块链上创建质押",
	}

	stake2FalgAutoRestakeUsages = map[config.Language]string{
		config.English: "auto restake without power decay",
		config.Chinese: "自动质押，权重不会衰减",
	}
)

// stake2CreateCmd represents the stake2 create command
var stake2CreateCmd = &cobra.Command{Use: config.TranslateInLang(stake2CreateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(stake2CreateCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(3, 4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Create(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stake2CreateCmd)
	stake2CreateCmd.Flags().BoolVar(&stake2AutoRestake, "auto-restake", false,
		config.TranslateInLang(stake2FalgAutoRestakeUsages, config.UILanguage))
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

	data := []byte{}
	if len(args) == 4 {
		data = make([]byte, 2*len([]byte(args[3])))
		hex.Encode(data, []byte(args[3]))
	}

	sender, err := signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := gasLimitFlag.Value().(uint64)
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

	s2c, err := action.NewCreateStake(nonce, candidateName, amountStringInRau, duration, stake2AutoRestake, data, gasLimit, gasPriceRau)
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
