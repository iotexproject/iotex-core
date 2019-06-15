// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
)

// xrc20TransferCmd could do transfer action
var xrc20TransferCmd = &cobra.Command{
	Use: "transfer (ALIAS|TARGET_ADDRESS) AMOUNT" +
		" -c ALIAS|CONTRACT_ADDRESS [-l GAS_LIMIT] -s SIGNER [-p GAS_PRICE]",
	Short: "Transfer token to the target address",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		recipient, err := alias.EtherAddress(args[0])
		if err != nil {
			return err
		}
		amount, ok := new(big.Int).SetString(args[1], 10)
		if !ok {
			return errors.Errorf("invalid XRC20 amount format %s", args[1])
		}
		bytecode, err := xrc20ABI.Pack("transfer", recipient, amount)
		if err != nil {
			return err
		}
		contract, err := xrc20Contract()
		if err != nil {
			return err
		}
		return execute(contract.String(), big.NewInt(0), bytecode)
	},
}

func init() {
	registerWriteCommand(xrc20TransferCmd)
}
