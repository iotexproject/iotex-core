// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// actionClaimCmd represents the action claim command
var actionClaimCmd = &cobra.Command{
	Use:   "claim AMOUNT_IOTX [DATA] -l GAS_LIMIT -p GASPRICE -s SIGNER",
	Short: "Claim rewards from rewarding fund",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(claim(args))
	},
}

// claim claims rewards from rewarding fund
func claim(args []string) string {
	amount, err := util.StringToRau(args[0], util.IotxDecimalNum)
	if err != nil {
		return err.Error()
	}
	payload := make([]byte, 0)
	if len(args) == 2 {
		payload = []byte(args[1])
	}
	sender, err := alias.Address(signer)
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
	b := &action.ClaimFromRewardingFundBuilder{}
	act := b.SetAmount(amount).SetData(payload).Build()
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(&act).Build()
	return sendAction(elp)
}
