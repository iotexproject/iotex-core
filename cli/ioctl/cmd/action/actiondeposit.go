// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disdeposited. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// actionDepositCmd represents the action deposit command
var actionDepositCmd = &cobra.Command{
	Use:   "deposit AMOUNT_IOTX [DATA] -s SIGNER [-l GAS_LIMIT] [-p GASPRICE]",
	Short: "Deposit rewards from rewarding fund",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := deposit(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// deposit deposits token into rewarding fund
func deposit(args []string) (string, error) {
	amount, err := util.StringToRau(args[0], util.IotxDecimalNum)
	if err != nil {
		return "", err
	}
	payload := make([]byte, 0)
	if len(args) == 2 {
		payload = []byte(args[1])
	}
	sender, err := alias.Address(signer)
	if err != nil {
		return "", err
	}
	if gasLimit == 0 {
		gasLimit = action.DepositToRewardingFundBaseGas +
			action.DepositToRewardingFundGasPerByte*uint64(len(payload))
	}
	var gasPriceRau *big.Int
	if len(gasPrice) == 0 {
		gasPriceRau, err = GetGasPrice()
		if err != nil {
			return "", err
		}
	} else {
		gasPriceRau, err = util.StringToRau(gasPrice, util.GasPriceDecimalNum)
		if err != nil {
			return "", err
		}
	}
	if nonce == 0 {
		accountMeta, err := account.GetAccountMeta(sender)
		if err != nil {
			return "", err
		}
		nonce = accountMeta.PendingNonce
	}
	b := &action.DepositToRewardingFundBuilder{}
	act := b.SetAmount(amount).SetData(payload).Build()
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(&act).Build()
	return sendAction(elp)
}
