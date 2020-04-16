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
	registerCmdUses = map[config.Language]string{
		config.English: "register NAME (ALIAS|OPERATO_ADDRESS) (ALIAS|REWARD_ADDRESS) (ALIAS|OWNER_ADDRESS) AMOUNT_IOTX STAKE_DURATION [DATA] [--auto-restake] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "register 姓名 (别名|操作者地址）（别名|奖励地址）（别名|所有者地址）IOTX数量 质押持续时间 [数据] [--auto-restake] [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	registerCmdShorts = map[config.Language]string{
		config.English: "register a candidate",
		config.Chinese: "注册候选人",
	}

	stake2FlagRegisterAutoRestakeUsages = map[config.Language]string{
		config.English: "auto restake without power decay",
		config.Chinese: "自动质押，权重不会衰减",
	}
)

// stake2RegisterCmd represents the stake2 register a candidate command
var stake2RegisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(registerCmdUses, config.UILanguage),
	Short: config.TranslateInLang(registerCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(6, 7),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := register(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stake2RegisterCmd)
	stake2RegisterCmd.Flags().BoolVar(&stake2AutoRestake, "auto-restake", false, config.TranslateInLang(stake2FlagRegisterAutoRestakeUsages, config.UILanguage))
}

func register(args []string) error {
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
	ownerAddrStr, err := util.Address(args[3])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get owner address", err)
	}

	amountInRau, err := util.StringToRau(args[4], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid amount", err)
	}

	stakeDuration, err := parseStakeDuration(args[5])
	if err != nil {
		return output.NewError(0, "", err)
	}
	duration := uint32(stakeDuration.Uint64())

	var payload []byte
	if len(args) == 7 {
		payload = make([]byte, 2*len([]byte(args[6])))
		hex.Encode(payload, []byte(args[6]))
	}

	sender, err := signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.CandidateRegisterBaseIntrinsicGas +
			action.CandidateRegisterPayloadGas*uint64(len(payload))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	cr, err := action.NewCandidateRegister(name, operatorAddrStr, rewardAddrStr, ownerAddrStr, amountInRau.String(), duration, stake2AutoRestake, payload)

	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a candidateRegister instance", err)
	}

	elp, err := (&action.EnvelopeBuilder{}).
		SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(cr).Build()
	if err != nil {
		return output.NewError(0, "failed to create envelope", err)
	}

	return SendAction(elp, sender)
}
