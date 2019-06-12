// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
)

// Xrc20TransferCmd could do transfer action
var Xrc20TransferCmd = &cobra.Command{
	Use: "transfer (ALIAS|TARGET_ADDRESS) (AMOUNT)" +
		" -c ALIAS|CONTRACT_ADDRESS -l GAS_LIMIT -s SIGNER [-p GAS_PRICE]",
	Short: "Get account balance",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		addr, err := alias.Address(args[0])
		if err != nil {
			return err
		}
		xrc20TargetAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		transfer, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return err
		}
		xrc20TransferAmount = uint64(transfer)
		output, err := transferTo(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// read reads smart contract on IoTeX blockchain
func transferTo(args []string) (string, error) {
	args[0] = xrc20ContractAddress
	args[1] = "0"
	var err error
	xrc20Bytes, err = xrc20ABI.Pack("transfer", toEthAddr(xrc20TargetAddress), new(big.Int).SetUint64(xrc20TransferAmount))
	if err != nil {
		return "", err
	}
	return invoke(args)
}
