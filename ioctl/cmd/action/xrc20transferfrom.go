// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/ioctl/output"
)

// xrc20TransferFromCmd could transfer from owner address to target address
var xrc20TransferFromCmd = &cobra.Command{
	Use: "transferFrom (ALIAS|OWNER_ADDRESS) (ALIAS|RECIPIENT_ADDRESS) AMOUNT -c (ALIAS|CONTRACT_ADDRESS)" +
		" [-s SIGNER] [-n NONCE] [-l GAS_LIMIT] [-p GAS_PRICE] [-P PASSWORD] [-y]",
	Short: "Send amount of tokens from owner address to target address",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := transferFrom(args)
		return output.PrintError(err)
	},
}

func init() {
	registerWriteCommand(xrc20TransferFromCmd)
}

func transferFrom(args []string) error {
	owner, err := alias.EtherAddress(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get owner address", err)
	}
	recipient, err := alias.EtherAddress(args[1])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get recipient address", err)
	}
	contract, err := xrc20Contract()
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}
	amount, err := parseAmount(contract, args[2])
	if err != nil {
		return output.NewError(0, "failed to parse amount", err)
	}
	bytecode, err := xrc20ABI.Pack("transferFrom", owner, recipient, amount)
	if err != nil {
		return output.NewError(output.ConvertError, "cannot generate bytecode from given command", err)
	}
	return Execute(contract.String(), big.NewInt(0), bytecode)
}
