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
	"github.com/iotexproject/iotex-core/ioctl/validator"
)

// Multi-language support
var (
	_stake2ChangeCmdUses = map[config.Language]string{
		config.English: "change CANDIDATE_NAME BUCKET_INDEX [DATA]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "change 候选人名字 票索引 [数据]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_stake2ChangeCmdShorts = map[config.Language]string{
		config.English: "Change stake candidate",
		config.Chinese: "在IoTeX区块链上改变投票候选人",
	}
)

// _stake2ChangeCmd represents the stake2 change command
var _stake2ChangeCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2ChangeCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2ChangeCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Change(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_stake2ChangeCmd)
}

func stake2Change(args []string) error {
	var candidateName = args[0]
	if err := validator.ValidateCandidateNameForStake2(candidateName); err != nil {
		return output.NewError(output.ValidationError, "invalid candidate name", err)
	}

	bucketIndex, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	var data []byte
	if len(args) == 3 {
		data, err = hex.DecodeString(args[2])
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
		gasLimit = action.MoveStakeBaseIntrinsicGas + action.MoveStakePayloadGas*uint64(len(data))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	s2c, err := action.NewChangeCandidate(nonce, candidateName, bucketIndex, data, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a changeCandidate instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2c).Build(),
		sender)
}
