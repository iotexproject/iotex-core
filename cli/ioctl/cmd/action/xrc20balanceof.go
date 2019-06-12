// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
)

// Xrc20BalanceofCmd represents balanceof function
var Xrc20BalanceofCmd = &cobra.Command{
	Use: "balanceof (ALIAS|OWNER_ADDRESS)" +
		" -c ALIAS|CONTRACT_ADDRESS ",
	Short: "Get account balance",
	Args:  cobra.ExactArgs(1),
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
		output, err := balanceOf(args)
		if err == nil {
			fmt.Println(output)
			result, _ := strconv.ParseUint(output, 16, 64)
			fmt.Println("Output in decimal format:")
			fmt.Println(uint64(result))
		}
		return err
	},
}

func toEthAddr(addr address.Address) common.Address {
	return common.BytesToAddress(addr.Bytes())
}

// read reads smart contract on IoTeX blockchain
func balanceOf(args []string) (string, error) {

	args[0] = contractAddress
	signer = "ALIAS"
	gasLimit = 50000
	var err error
	bytes, err = abiResult.Pack("balanceOf", toEthAddr(ownerAddress))
	if err != nil {
		return "", err
	}
	return read(args)
}
