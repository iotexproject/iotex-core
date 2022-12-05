// Copyright (c) 2022 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_stake2WithDrawCmdUses = map[config.Language]string{
		config.English: "withdraw BUCKET_INDEX [DATA] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "withdraw 票索引 [数据] [-s 签署人] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_stake2WithDrawCmdShorts = map[config.Language]string{
		config.English: "Withdraw bucket from IoTeX blockchain",
		config.Chinese: "提取IoTeX区块链上的投票",
	}
)

// NewStake2WithdrawCmd represents the stake2 withdraw command
func NewStake2WithdrawCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_stake2WithDrawCmdUses)
	short, _ := client.SelectTranslation(_stake2WithDrawCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			bucketIndex, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return errors.Wrap(err, "failed to convert bucket index")
			}

			var data []byte
			if len(args) == 2 {
				data, err = hex.DecodeString(args[1])
				if err != nil {
					return errors.Wrap(err, "failed to decode data")
				}
			}

			gasPrice, signer, password, nonce, gasLimit, assumeYes, err := GetWriteCommandFlag(cmd)
			if err != nil {
				return err
			}
			sender, err := Signer(client, signer)
			if err != nil {
				return errors.Wrap(err, "failed to get signed address")
			}
			if gasLimit == 0 {
				gasLimit = action.ReclaimStakeBaseIntrinsicGas + action.ReclaimStakePayloadGas*uint64(len(data))
			}
			gasPriceRau, err := gasPriceInRau(client, gasPrice)
			if err != nil {
				return errors.Wrap(err, "failed to get gas price")
			}
			nonce, err = checkNonce(client, nonce, sender)
			if err != nil {
				return errors.Wrap(err, "failed to get nonce")
			}
			s2w, err := action.NewWithdrawStake(nonce, bucketIndex, data, gasLimit, gasPriceRau)
			if err != nil {
				return errors.Wrap(err, "failed to make a changeCandidate instance")
			}
			return SendAction(
				client,
				cmd,
				(&action.EnvelopeBuilder{}).
					SetNonce(nonce).
					SetGasPrice(gasPriceRau).
					SetGasLimit(gasLimit).
					SetAction(s2w).Build(),
				sender,
				password,
				nonce,
				assumeYes,
			)
		},
	}
	RegisterWriteCommand(client, cmd)
	return cmd
}
