// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is didslaimed. This source code is governed by Apache
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
	stake2AddCmdUses = map[config.Language]string{
		config.English: "add BUCKET_INDEX AMOUNT_IOTX [DATA]" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "add 票索引 IOTX数量 [数据]" +
			" [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}

	stake2AddCmdShorts = map[config.Language]string{
		config.English: "Add IOTX to bucket on IoTeX blockchain",
		config.Chinese: "添加IOTX到IoTeX区块链上的投票",
	}
)

// stake2AddCmd represents the stake2 add command
var stake2AddCmd = &cobra.Command{
	Use:   config.TranslateInLang(stake2AddCmdUses, config.UILanguage),
	Short: config.TranslateInLang(stake2AddCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := stake2Add(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(stake2AddCmd)
}

func stake2Add(args []string) error {

	bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return output.NewError(output.ConvertError, "failed to convert bucket index", nil)
	}

	amountInRau, err := util.StringToRau(args[1], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid amount", err)
	}

	data := []byte{}
	if len(args) == 3 {
		data = make([]byte, 2*len([]byte(args[2])))
		hex.Encode(data, []byte(args[2]))
	}

	sender, err := signer()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get signed address", err)
	}

	gasLimit := gasLimitFlag.Value().(uint64)
	if gasLimit == 0 {
		gasLimit = action.DepositToStakeBaseIntrinsicGas + action.DepositToStakePayloadGas*uint64(len(data))
	}

	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}

	s2a, err := action.NewDepositToStake(nonce, bucketIndex, amountInRau.String(), data, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a depositToStake instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(s2a).Build(),
		sender)
}
