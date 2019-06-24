// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// actionTransferCmd represents the action transfer command
var actionTransferCmd = &cobra.Command{
	Use: "transfer (ALIAS|RECIPIENT_ADDRESS) AMOUNT_IOTX [DATA]" +
		" -s SIGNER [-l GAS_LIMIT] [-p GAS_PRICE]",
	Short: "Transfer tokens on IoTeX blokchain",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		recipient, err := alias.Address(args[0])
		if err != nil {
			return err
		}
		amount, err := util.StringToRau(args[1], util.IotxDecimalNum)
		if err != nil {
			return err
		}
		var payload []byte
		if len(args) == 3 {
			payload, err = hex.DecodeString(args[2])
			if err != nil {
				return err
			}
		}
		sender, err := signer()
		if err != nil {
			return err
		}
		gasLimit := gasLimitFlag.Value().(uint64)
		if gasLimit == 0 {
			gasLimit = action.TransferBaseIntrinsicGas +
				action.TransferPayloadGas*uint64(len(payload))
		}
		gasPriceRau, err := gasPriceInRau()
		if err != nil {
			return err
		}
		nonce, err := nonce(sender)
		if err != nil {
			return err
		}
		tx, err := action.NewTransfer(nonce, amount,
			recipient, payload, gasLimit, gasPriceRau)
		if err != nil {
			log.L().Error("failed to make a Transfer instance", zap.Error(err))
			return err
		}
		address, err := alias.Address(sender)
		if err != nil {
			return err
		}
		accountMeta, err := account.GetAccountMeta(address)
		if err != nil {
			return err
		}
		balance, ok := big.NewInt(0).SetString(accountMeta.Balance, 10)
		if !ok {
			return fmt.Errorf("failed to convert balance into big int")
		}
		cost, err := tx.Cost()
		if err != nil {
			return err
		}
		if balance.Cmp(cost) < 0 {
			return fmt.Errorf("balance is not enough")
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
