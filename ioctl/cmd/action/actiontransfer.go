// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// actionTransferCmd represents the action transfer command
var actionTransferCmd = &cobra.Command{
	Use: "transfer (ALIAS|RECIPIENT_ADDRESS) AMOUNT_IOTX [DATA]" +
		" [-s SIGNER] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
	Short: "Transfer tokens on IoTeX blokchain",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		recipient, err := util.Address(args[0])
		if err != nil {
			return output.PrintError(output.AddressError, err.Error())
		}
		amount, err := util.StringToRau(args[1], util.IotxDecimalNum)
		if err != nil {
			return output.PrintError(output.ConvertError, err.Error())
		}
		var payload []byte
		if len(args) == 3 {
			payload, err = hex.DecodeString(args[2])
			if err != nil {
				return output.PrintError(output.ConvertError, err.Error())
			}
		}
		sender, err := signer()
		if err != nil {
			return output.PrintError(output.AddressError, err.Error())
		}
		gasLimit := gasLimitFlag.Value().(uint64)
		if gasLimit == 0 {
			gasLimit = action.TransferBaseIntrinsicGas +
				action.TransferPayloadGas*uint64(len(payload))
		}
		gasPriceRau, err := gasPriceInRau()
		if err != nil {
			return output.PrintError(output.ConvertError, err.Error())
		}
		nonce, err := nonce(sender)
		if err != nil {
			return output.PrintError(0, err.Error()) //TODO: undefined error
		}
		tx, err := action.NewTransfer(nonce, amount,
			recipient, payload, gasLimit, gasPriceRau)
		if err != nil {
			return output.PrintError(0, "failed to make a Transfer instance"+err.Error()) // TODO: undefined error
		}
		return sendAction(
			(&action.EnvelopeBuilder{}).
				SetNonce(nonce).
				SetGasPrice(gasPriceRau).
				SetGasLimit(gasLimit).
				SetAction(tx).Build(),
			sender,
		)
	},
}

func init() {
	registerWriteCommand(actionTransferCmd)
}
