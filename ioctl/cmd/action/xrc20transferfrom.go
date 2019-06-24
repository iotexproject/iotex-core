// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
)

// xrc20TransferFromCmd could transfer from owner address to target address
var xrc20TransferFromCmd = &cobra.Command{
	Use: "transferFrom (ALIAS|OWNER_ADDRESS) (ALIAS|RECIPIENT_ADDRESS) AMOUNT" +
		" -c (ALIAS|CONTRACT_ADDRESS) -s SIGNER [-l GAS_LIMIT] [-p GAS_PRICE]",
	Short: "Send amount of tokens from owner address to target address",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		owner, err := alias.EtherAddress(args[0])
		if err != nil {
			return err
		}
		recipient, err := alias.EtherAddress(args[1])
		if err != nil {
			return err
		}
		contract, err := xrc20Contract()
		if err != nil {
			return err
		}
		amount, err := parseAmount(contract, args[2])
		if err != nil {
			return err
		}
		bytecode, err := xrc20ABI.Pack("transferFrom", owner, recipient, amount)
		if err != nil {
			return err
		}
		return execute(contract.String(), nil, bytecode)
	},
}

func init() {
	registerWriteCommand(xrc20TransferFromCmd)
}
