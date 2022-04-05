// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disdeposited. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_depositCmdShorts = map[config.Language]string{
		config.English: "Deposit rewards to rewarding fund",
		config.Chinese: "将奖励存入奖励基金",
	}
	_depositCmdUses = map[config.Language]string{
		config.English: "deposit AMOUNT_IOTX [DATA] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "deposit IOTX数量 [数据] [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P" +
			" 密码] [-y]",
	}
)

// _actionDepositCmd represents the action deposit command
var _actionDepositCmd = &cobra.Command{
	Use:   config.TranslateInLang(_depositCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_depositCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := deposit(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_actionDepositCmd)
}

func deposit(args []string) error {
	amount, err := util.StringToRau(args[0], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid amount", err)
	}
	payload := make([]byte, 0)
	if len(args) == 2 {
		payload = []byte(args[1])
	}
	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.DepositToRewardingFundBaseGas +
			action.DepositToRewardingFundGasPerByte*uint64(len(payload))
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gasPriceRau", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce", err)
	}
	act := (&action.DepositToRewardingFundBuilder{}).SetAmount(amount).SetData(payload).Build()

	return SendAction((&action.EnvelopeBuilder{}).SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(&act).Build(),
		sender,
	)
}
