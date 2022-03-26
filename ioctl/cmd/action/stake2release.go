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
	_stake2ReleaseCmdUses = map[config.Language]string{
		config.English: "release BUCKET_INDEX [DATA]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "release 票索引 [数据]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_stake2ReleaseCmdShorts = map[config.Language]string{
		config.English: "Release bucket on IoTeX blockchain",
		config.Chinese: "撤回IoTeX区块链上的投票",
	}
)

// _stake2ReleaseCmd represents the stake2 release command
var _stake2ReleaseCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2ReleaseCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2ReleaseCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Release(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_stake2ReleaseCmd)
}

func stake2Release(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	var data []byte
	if len(args) == 2 {
		data, err = hex.DecodeString(args[1])
		if err != nil {
			return output.NewError(output.ConvertError, "failed to decode data", err)
		}
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	sender, err := Signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signer address", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce", err)
	}
	gasLimit := _gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.ReclaimStakeBaseIntrinsicGas + action.ReclaimStakePayloadGas*uint64(len(data))
	}
	s2r, err := action.NewUnstake(nonce, bucketIndex, data, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a Unstake  instance", err)
	}

	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2r).Build(),
		sender)
}
