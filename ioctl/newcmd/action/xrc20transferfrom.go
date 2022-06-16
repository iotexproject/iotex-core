// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/alias"
)

// Multi-language support
var (
	_xrc20TransferFromCmdUses = map[config.Language]string{
		config.English: "transferFrom (ALIAS|OWNER_ADDRESS) (ALIAS|RECIPIENT_ADDRESS) AMOUNT -c (ALIAS|CONTRACT_ADDRESS)" +
			" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
		config.Chinese: "transferFrom (别名|所有人地址) (别名|接收人地址) 数量 -c (" +
			"别名|合约地址)" +
			" [-s 签署] [-n NONCE] [-l GAS限制] [-p GAS价格] [-P 密码] [-y]",
	}
	_xrc20TransferFromCmdShorts = map[config.Language]string{
		config.English: "Send amount of tokens from owner address to target address",
		config.Chinese: "将通证数量从所有者地址发送到目标地址",
	}
)

// NewXrc20TransferFrom represent xrc20TransferFrom command
func NewXrc20TransferFrom(client ioctl.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   selectTranslation(client, _xrc20TransferFromCmdUses),
		Short: selectTranslation(client, _xrc20TransferFromCmdShorts),
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			owner, err := alias.EtherAddress(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to get owner address")
			}
			recipient, err := alias.EtherAddress(args[1])
			if err != nil {
				return errors.Wrap(err, "failed to get recipient address")
			}
			contract, err := xrc20Contract(cmd)
			if err != nil {
				return errors.Wrap(err, "failed to get contract address")
			}
			amount, err := parseAmount(client, cmd, contract, args[2])
			if err != nil {
				return errors.Wrap(err, "failed to parse amount")
			}
			bytecode, err := ioctl.PackABI(ioctl.XRC20ABI, "transferFrom", owner, recipient, amount)
			if err != nil {
				return errors.Wrap(err, "cannot generate bytecode from given command")
			}
			err = Execute(client, cmd, contract.String(), big.NewInt(0), bytecode)
			if err != nil {
				cmd.PrintErr(err)
			}
			return err
		},
	}
	RegisterWriteCommand(client, cmd)
	return cmd
}
