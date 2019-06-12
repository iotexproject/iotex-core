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

// Xrc20TransferFromCmd could transfer from owner address to target address
var Xrc20TransferFromCmd = &cobra.Command{
	Use: "transferfrom (ALIAS|OWNER_ADDRESS) (ALIAS|TARGET_ADDRESS) (AMOUNT)" +
		" -c (ALIAS|CONTRACT_ADDRESS) -l GAS_LIMIT -s SIGNER [-p GAS_PRICE]",
	Short: "Send amount of tokens from owner address to target address",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		addr, err := alias.Address(args[0])
		if err != nil {
			return err
		}
		ownerAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		addr, err = alias.Address(args[1])
		if err != nil {
			return err
		}
		targetAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		transferAmount, err = strconv.ParseUint(args[2], 10, 64)
		if err != nil {
			return err
		}
		output, err := transferFrom(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// read reads smart contract on IoTeX blockchain
func transferFrom(args []string) (string, error) {
	args[0] = contractAddress
	args[1] = "0"
	var err error
	bytes, err = abiResult.Pack("transferFrom", toEthAddr(ownerAddress), toEthAddr(targetAddress), new(big.Int).SetUint64(transferAmount))
	if err != nil {
		return "", err
	}
	return invoke(args)
}
