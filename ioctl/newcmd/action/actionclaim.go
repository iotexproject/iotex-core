// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/ioctl"
	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/flag"
	"github.com/iotexproject/iotex-core/v2/ioctl/output"
)

// Multi-language support
var (
	_claimCmdShorts = map[config.Language]string{
		config.English: "Claim rewards from rewarding fund",
		config.Chinese: "从奖励基金中获取奖励",
	}
	_claimCmdUses = map[config.Language]string{
		config.English: "claim --amount AMOUNT_IOTX [-address ACCOUNT_REWARD_TO] [--payload DATA] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "claim --amount IOTX数量 [--address 获取奖励的账户地址] [--payload 数据] [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
)

// flags
var (
	claimAmount  = flag.NewStringVarP("amount", "", "", config.TranslateInLang(_flagClaimAmount, config.UILanguage))
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

// NewActionClaimCmd represents the action claim command
func NewActionClaimCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_claimCmdUses)
	short, _ := client.SelectTranslation(_claimCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			amount, ok := new(big.Int).SetString(claimAmount.Value().(string), 10)
			if !ok {
				return output.PrintError(errors.Errorf("invalid amount: %s", claimAmount))
			}
			if amount.Cmp(new(big.Int).SetInt64(0)) < 1 {
				return output.PrintError(errors.Errorf("expect amount greater than 0, but got: %v", amount.Int64()))
			}

			gasPrice, signer, password, nonce, gasLimit, assumeYes, err := GetWriteCommandFlag(cmd)
			if err != nil {
				return err
			}

			sender, err := Signer(client, signer)
			if err != nil {
				return errors.Wrap(err, "failed to get signer address")
			}
			var addr address.Address
			if s := claimAddress.Value().(string); len(s) > 0 {
				addr, err = address.FromString(s)
				if err != nil {
					return errors.Wrap(err, "failed to decode claim address")
				}
			}
			payload := []byte(claimPayload.Value().(string))
			if gasLimit == 0 {
				gasLimit = action.ClaimFromRewardingFundBaseGas +
					action.ClaimFromRewardingFundGasPerByte*uint64(len(payload))
			}
			gasPriceRau, err := gasPriceInRau(client, gasPrice)
			if err != nil {
				return errors.Wrap(err, "failed to get gasPriceRau")
			}
			nonce, err = checkNonce(client, nonce, sender)
			if err != nil {
				return errors.Wrap(err, "failed to get nonce")
			}
			act := action.NewClaimFromRewardingFund(amount, addr, payload)
			return SendAction(
				client,
				cmd,
				(&action.EnvelopeBuilder{}).SetNonce(nonce).
					SetGasPrice(gasPriceRau).
					SetGasLimit(gasLimit).
					SetAction(act).Build(),
				sender,
				password,
				nonce,
				assumeYes,
			)
		},
	}

	claimAmount.RegisterCommand(cmd)
	claimAddress.RegisterCommand(cmd)
	claimPayload.RegisterCommand(cmd)

	_ = cmd.MarkFlagRequired("amount")

	RegisterWriteCommand(client, cmd)

	return cmd
}
