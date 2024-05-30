// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"math/big"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/flag"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_claimCmdShorts = map[config.Language]string{
		config.English: "Claim rewards from rewarding fund",
		config.Chinese: "从奖励基金中获取奖励",
	}
	_claimCmdUses = map[config.Language]string{
		config.English: "claim AMOUNT_IOTX [ACCOUNT_REWARD_TO] [DATA] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "claim IOTX数量 [获取奖励的账户地址] [数据] [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
)

// flags
var (
	claimAmount  = flag.NewStringVarP("amount", "", "0", config.TranslateInLang(_flagClaimAmount, config.UILanguage))
	claimPayload = flag.NewStringVarP("payload", "", "", config.TranslateInLang(_flagClaimPayload, config.UILanguage))
	claimAddress = flag.NewStringVarP("address", "", "", config.TranslateInLang(_flagClaimAddress, config.UILanguage))
)

// flag multi-language
var (
	_flagClaimAmount = map[config.Language]string{
		config.English: "amount of IOTX, default 0, unit RAU",
		config.Chinese: "IOTX数量",
	}
	_flagClaimPayload = map[config.Language]string{
		config.English: "claim reward action payload data",
		config.Chinese: "action数据",
	}
	_flagClaimAddress = map[config.Language]string{
		config.English: "address of claim reward to, default is the action sender address",
		config.Chinese: "获取奖励的账户地址, 默认使用action发送者地址",
	}
)

// _actionClaimCmd represents the action claim command
var _actionClaimCmd = &cobra.Command{
	Use:   config.TranslateInLang(_claimCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_claimCmdShorts, config.UILanguage),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		amount, ok := new(big.Int).SetString(claimAmount.Value().(string), 10)
		if !ok {
			return output.PrintError(errors.Errorf("invalid amount: %s", claimAmount))
		}
		err := claim(amount, claimPayload.Value().(string), claimAddress.Value().(string))
		return output.PrintError(err)
	},
}

func init() {
	claimAmount.RegisterCommand(_actionClaimCmd)
	claimPayload.RegisterCommand(_actionClaimCmd)
	claimAddress.RegisterCommand(_actionClaimCmd)
	RegisterWriteCommand(_actionClaimCmd)
}

func claim(amount *big.Int, payload, address string) error {
	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	if address == "" {
		address = sender
	}

	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.ClaimFromRewardingFundBaseGas +
			action.ClaimFromRewardingFundGasPerByte*uint64(len(payload))
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gasPriceRau", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce", err)
	}
	act := (&action.ClaimFromRewardingFundBuilder{}).
		SetAmount(amount).
		SetData([]byte(payload)).
		SetAddress(address).
		Build()

	return SendAction((&action.EnvelopeBuilder{}).SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(&act).Build(),
		sender,
	)
}
