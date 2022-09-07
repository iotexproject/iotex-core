// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/account"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_actionTransferCmdShorts = map[config.Language]string{
		config.English: "Transfer tokens on IoTeX blokchain",
		config.Chinese: "在IoTeX区块链上转移令牌",
	}
	_actionTransferCmdUses = map[config.Language]string{
		config.English: "transfer (ALIAS|RECIPIENT_ADDRESS) AMOUNT_IOTX [DATA] [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "transfer (别名|接收人地址) IOTX数量 [数据] [-s 签署人] [-n NONCE] [-l GAS限制] [-P GAS" +
			"价格] [-P 密码] [-y]",
	}
)

// NewActionTransferCmd represents the action transfer command
func NewActionTransferCmd(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_actionTransferCmdUses)
	short, _ := client.SelectTranslation(_actionTransferCmdShorts)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			recipient, err := client.Address(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get recipient address")
			}

			accountMeta, err := account.Meta(client, recipient)
			if err != nil {
				return errors.Wrap(err, "failed to get account meta")
			}
			if accountMeta.IsContract {
				return errors.New("use 'ioctl contract' command instead")
			}

			amount, err := util.StringToRau(args[1], util.IotxDecimalNum)
			if err != nil {
				return errors.Wrap(err, "invalid amount")
			}
			var payload []byte
			if len(args) == 3 {
				payload, err = hex.DecodeString(args[2])
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
				gasLimit = action.TransferBaseIntrinsicGas + action.TransferPayloadGas*uint64(len(payload))
			}
			gasPriceRau, err := gasPriceInRau(client, gasPrice)
			if err != nil {
				return errors.Wrap(err, "failed to get gas price")
			}
			nonce, err = checkNonce(client, nonce, sender)
			if err != nil {
				return errors.Wrap(err, "failed to get nonce")
			}
			tx, err := action.NewTransfer(nonce, amount, recipient, payload, gasLimit, gasPriceRau)
			if err != nil {
				return errors.Wrap(err, "failed to make a Transfer instance")
			}
			return SendAction(
				client,
				cmd,
				(&action.EnvelopeBuilder{}).
					SetNonce(nonce).
					SetGasPrice(gasPriceRau).
					SetGasLimit(gasLimit).
					SetAction(tx).Build(),
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
