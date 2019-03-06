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

// actionTransferCmd transfers tokens on IoTeX blockchain
var actionTransferCmd = &cobra.Command{
	Use:   "transfer recipient amount data",
	Short: "Transfer tokens on IoTeX blokchain",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(transfer(args))
	},
}

// transfer transfers tokens on IoTeX blockchain
func transfer(args []string) string {
	// Validate inputs
	if err := validator.ValidateAddress(args[0]); err != nil {
		return err.Error()
	}
	recipient := args[0]
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		log.L().Error("cannot convert "+args[1]+" into int64", zap.Error(err))
		return err.Error()
	}
	if err := validator.ValidateAmount(amount); err != nil {
		return err.Error()
	}
	payload := args[2]

	sender, err := account.AliasToAddress(alias)
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
