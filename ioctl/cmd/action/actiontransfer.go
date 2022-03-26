// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	__actionTransferCmdShorts = map[config.Language]string{
		config.English: "Transfer tokens on IoTeX blokchain",
		config.Chinese: "在IoTeX区块链上转移令牌",
	}
	__actionTransferCmdUses = map[config.Language]string{
		config.English: "transfer (ALIAS|RECIPIENT_ADDRESS) AMOUNT_IOTX [DATA] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "transfer (别名|接收人地址) IOTX数量 [数据] [-s 签署人] [-n NONCE] [-l GAS限制] [-P GAS" +
			"价格] [-P 密码] [-y]",
	}
)

// _actionTransferCmd represents the action transfer command
var _actionTransferCmd = &cobra.Command{
	Use:   config.TranslateInLang(__actionTransferCmdUses, config.UILanguage),
	Short: config.TranslateInLang(__actionTransferCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := transfer(args)
		return output.PrintError(err)
	},
}

func init() {
	RegisterWriteCommand(_actionTransferCmd)
}

func transfer(args []string) error {
	recipient, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get recipient address", err)
	}

	accountMeta, err := account.GetAccountMeta(recipient)
	if err != nil {
		return output.NewError(0, "failed to get account meta", err)
	}
	if accountMeta.IsContract {
		return output.NewError(output.RuntimeError, "use 'ioctl contract' command instead", err)
	}

	amount, err := util.StringToRau(args[1], util.IotxDecimalNum)
	if err != nil {
		return output.NewError(output.ConvertError, "invalid amount", err)
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
		gasLimit = action.TransferBaseIntrinsicGas +
			action.TransferPayloadGas*uint64(len(payload))
	}
	gasPriceRau, err := gasPriceInRau()
	if err != nil {
		return output.NewError(0, "failed to get gas price", err)
	}
	nonce, err := nonce(sender)
	if err != nil {
		return output.NewError(0, "failed to get nonce ", err)
	}
	tx, err := action.NewTransfer(nonce, amount,
		recipient, payload, gasLimit, gasPriceRau)
	if err != nil {
		return output.NewError(output.InstantiationError, "failed to make a Transfer instance", err)
	}
	return SendAction(
		(&action.EnvelopeBuilder{}).
			SetNonce(nonce).
			SetGasPrice(gasPriceRau).
			SetGasLimit(gasLimit).
			SetAction(tx).Build(),
		sender,
	)

}
