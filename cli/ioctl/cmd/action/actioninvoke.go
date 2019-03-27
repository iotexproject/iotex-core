// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
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

// actionInvokeCmd represents the action invoke command
var actionInvokeCmd = &cobra.Command{
	Use: "invoke (ALIAS|CONTRACT_ADDRESS) [AMOUNT_IOTX]" +
		" -l GAS_LIMIT -p GAS_PRICE -s SIGNER -b BYTE_CODE",
	Short: "Invoke smart contract on IoTeX blockchain",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(invoke(args))
	},
}

// invoke invokes smart contract on IoTeX blockchain
func invoke(args []string) string {
	contract, err := alias.Address(args[0])
	if err != nil {
		return err.Error()
	}
	amount := big.NewInt(0)
	if len(args) == 2 {
		amount, err = util.StringToRau(args[1], util.IotxDecimalNum)
		if err != nil {
			return err.Error()
		}
	}
	executor, err := alias.Address(signer)
	if err != nil {
		return err.Error()
	}
	gasPriceRau, err := util.StringToRau(gasPrice, util.GasPriceDecimalNum)
	if err != nil {
		return err.Error()
	}
	if nonce == 0 {
		accountMeta, err := account.GetAccountMeta(executor)
		if err != nil {
			return err.Error()
		}
		nonce = accountMeta.PendingNonce
	}
	tx, err := action.NewExecution(contract, nonce, amount, gasLimit, gasPriceRau, bytecode)
	if err != nil {
		log.L().Error("cannot make a Execution instance", zap.Error(err))
		return err.Error()
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(tx).Build()
	return sendAction(elp)
}
