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
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_stake2TransferCmdUses = map[config.Language]string{
		config.English: "transfer (ALIAS|VOTE_ADDRESS) BUCKET_INDEX [DATA]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "transfer (别名|投票地址) 票索引 [数据]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	_stake2TransferCmdShorts = map[config.Language]string{
		config.English: "Transfer bucket ownership on IoTeX blockchain",
		config.Chinese: "在IoTeX区块链上转移投票所有权",
	}
)

// _stake2TransferCmd represents the stake2 transfer command
var _stake2TransferCmd = &cobra.Command{
	Use:   config.TranslateInLang(_stake2TransferCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_stake2TransferCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Transfer(args)
		return output.PrintError(err)

	},
}

func init() {
	RegisterWriteCommand(_stake2TransferCmd)
}

func stake2Transfer(args []string) error {
	voterAddrStr, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get voter address", err)
	}

	bucketIndex, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

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
		gasLimit = action.MoveStakeBaseIntrinsicGas +
			action.MoveStakePayloadGas*uint64(len(payload))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	s2t, err := action.NewTransferStake(nonce, voterAddrStr, bucketIndex, payload, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a transferStake instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2t).Build(),
		sender)
}
