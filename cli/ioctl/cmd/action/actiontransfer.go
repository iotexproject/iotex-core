// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// actionTransferCmd represents the action transfer command
var actionTransferCmd = &cobra.Command{
	Use:   "transfer recipient amount data",
	Short: "Transfer tokens on IoTeX blokchain",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(transfer(args))
	},
}

func init() {
	actionTransferCmd.Flags().Uint64VarP(&gasLimit, "gas-limit", "l", 0, "set gas limit")
	actionTransferCmd.Flags().Int64VarP(&gasPrice, "gas-price", "p", 0, "set gas prize")
	actionTransferCmd.Flags().StringVarP(&alias, "alias", "a", "", "choose signing key")
	if err := actionTransferCmd.MarkFlagRequired("gas-limit"); err != nil {
		log.L().Error(err.Error())
	}
	if err := actionTransferCmd.MarkFlagRequired("gas-price"); err != nil {
		log.L().Error(err.Error())
	}
	if err := actionTransferCmd.MarkFlagRequired("alias"); err != nil {
		log.L().Error(err.Error())
	}
}

// transfer transfers tokens on IoTeX blockchain
func transfer(args []string) string {
	recipient, err := account.Address(args[0])
	if err != nil {
		return err.Error()
	}
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		log.L().Error("cannot convert "+args[1]+" into int64", zap.Error(err))
		return err.Error()
	}
	if err := validator.ValidateAmount(amount); err != nil {
		return err.Error()
	}
	payload := args[2]

	sender, err := account.Address(signer)
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
	tx, err := action.NewTransfer(nonce, big.NewInt(amount),
		recipient, []byte(payload), gasLimit, big.NewInt(gasPrice))
	if err != nil {
		log.L().Error("cannot make a Transfer instance", zap.Error(err))
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(big.NewInt(gasPrice)).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	return sendAction(elp)
}
