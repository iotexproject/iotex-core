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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// actionSetRewardCmd represents the action setreward command
var actionSetRewardCmd = &cobra.Command{
	Use: "setreward AMOUNT_IOTX REWARD_TYPE DATA",
	Short: "Set reward for producers on IoTeX blockchain. " +
		"Reward types: 0 (block reward), 1 (epoch reward)",
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(setReward(args))
	},
}

// setReward sets reward for producers on IoTeX blockchain
func setReward(args []string) string {
	amount := big.NewInt(0)
	var err error
	amount, err = util.StringToRau(args[0], util.IotxDecimalNum)
	if err != nil {
		return err.Error()
	}
	var rewardType int
	switch args[1] {
	case "0":
		rewardType = action.BlockReward
	case "1":
		rewardType = action.EpochReward
	default:
		return "invalid REWARD_TYPE: 0 (block reward), 1 (epoch reward)"
	}
	payload := []byte(args[2])
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
	b := action.SetRewardBuilder{}
	act := b.SetAmount(amount).SetRewardType(rewardType).SetData(payload).Build()
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPriceRau).
		SetGasLimit(gasLimit).
		SetAction(&act).Build()
	return sendAction(elp)
}
