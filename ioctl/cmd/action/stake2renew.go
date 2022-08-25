// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// Multi-language support
var (
	_stake2RenewCmdUses = map[config.Language]string{
		config.English: "renew BUCKET_INDEX STAKE_DURATION [DATA] [--auto-stake]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "renew 票索引 投票持续时间 [数据] [--auto-stake]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	_stake2RenewCmdShorts = map[config.Language]string{
		config.English: "Renew bucket on IoTeX blockchain",
		config.Chinese: "更新IoTeX区块链上的投票",
	}
)

// _stake2RenewCmd represents the stake2 renew command
var _stake2RenewCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2RenewCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2RenewCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Renew(args)
		return output.PrintError(err)
	}}

func init() {
	RegisterWriteCommand(_stake2RenewCmd)
	_stake2RenewCmd.Flags().BoolVar(&_stake2AutoStake, "auto-stake", false,
		config.TranslateInLang(_stake2FlagAutoStakeUsages, config.UILanguage))
}

func stake2Renew(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	stakeDuration, err := parseStakeDuration(args[1])
	if err != nil {
		return output.NewError(0, "", err)
	}
	duration := uint32(stakeDuration.Uint64())

	var payload []byte
	if len(args) == 3 {
		payload, err = hex.DecodeString(args[2])
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
		gasLimit = action.RestakeBaseIntrinsicGas +
			action.RestakePayloadGas*uint64(len(payload))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	s2r, err := action.NewRestake(nonce, bucketIndex, duration, _stake2AutoStake, payload, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a restake instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2r).Build(),
		sender)
}
