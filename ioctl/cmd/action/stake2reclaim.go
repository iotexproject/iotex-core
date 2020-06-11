// Copyright (c) 2020 IoTeX Foundation
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
	stake2ReclaimCmdUses = map[config.Language]string{
		config.English: "reclaim BUCKET_INDEX SIGNATURE TYPE" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "reclaim 票索引 签名 类型" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	stake2ReclaimCmdShorts = map[config.Language]string{
		config.English: "Reclaim bucket on IoTeX blockchain",
		config.Chinese: "认领IoTeX区块链上的投票",
	}
)

// stake2ReclaimCmd represents the stake2 reclaim command
var stake2ReclaimCmd = &cobra.Command{
	Use:   config.TranslateInLang(stake2ReclaimCmdUses, config.UILanguage),
	Short: config.TranslateInLang(stake2ReclaimCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Reclaim(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(stake2ReclaimCmd)
}

func stake2Reclaim(args []string) error {
	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	sender, err := signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	if _, err = hex.DecodeString(args[1]); err != nil {
		return output.NewError(output.ConvertError, "failed to process signature", err)
	}

	// construct consignment JSON
	payload, err := action.NewConsignJSON(args[2], sender, args[1], bucketIndex, nonce)
	if err != nil {
		return output.NewError(output.InputError, "failed to create reclaim JSON", err)
	}

	gasLimit := gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.MoveStakeBaseIntrinsicGas +
			action.MoveStakePayloadGas*uint64(len(payload))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}

	s2t, err := action.NewTransferStake(nonce, sender, bucketIndex, payload, gasLimit, gasPriceRau)
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
