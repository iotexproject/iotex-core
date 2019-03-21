// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// actionTransferCmd represents the action transfer command
var actionTransferCmd = &cobra.Command{
	Use:   "transfer RECIPIENT AMOUNT_IOTX DATA -l GAS_LIMIT -p GAS_PRICE -s OPERATOR",
	Short: "Transfer tokens on IoTeX blokchain",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(transfer(args))
	},
}

// transfer transfers tokens on IoTeX blockchain
func transfer(args []string) string {
	recipient, err := account.Address(args[0])
	if err != nil {
		return err.Error()
	}
	amount, err := util.StringToRau(args[1], util.IotxDecimalNum)
	if err != nil {
		return err.Error()
	}
	payload := args[2]
	sender, err := account.Address(signer)
	if err != nil {
		return err.Error()
	}
	gasPriceRau, err := util.StringToRau(gasPrice, util.GasPriceDecimalNum)
	if err != nil {
		return err.Error()
	}
	if nonce == 0 {
		accountMeta, err := account.GetAccountMeta(sender)
		if err != nil {
			return err.Error()
		}
		nonce = accountMeta.PendingNonce
	}
	tx, err := action.NewTransfer(nonce, amount,
		recipient, []byte(payload), gasLimit, gasPriceRau)
	if err != nil {
		log.L().Error("cannot make a Transfer instance", zap.Error(err))
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	return sendAction(elp)
}
