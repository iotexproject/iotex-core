// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// Xrc20AllowanceCmd represents your signer limited amount on target address
var Xrc20AllowanceCmd = &cobra.Command{
	Use: "allowance (ALIAS|OWNER_ADDRESS) (ALIAS|SPENDER_ADDRESS) " +
		" -c ALIAS|CONTRACT_ADDRESS ",
	Short: "the amount which spender is still allowed to withdraw from owner",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		addr, err := alias.Address(args[0])
		if err != nil {
			return err
		}
		xrc20OwnerAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		addr, err = alias.Address(args[1])
		if err != nil {
			return err
		}
		xrc20SpenderAddress, err = address.FromString(addr)
		if err != nil {
			return err
		}
		output, err := allowance(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// read reads smart contract on IoTeX blockchain
func allowance(args []string) (string, error) {
	var err error
	args[0] = xrc20ContractAddress
	gasLimit = 50000
	signer = "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqd39ym7"
	xrc20Bytes, err = xrc20ABI.Pack("allowance", toEthAddr(xrc20OwnerAddress), toEthAddr(xrc20SpenderAddress))
	if err != nil {
		log.L().Error("cannot generate bytecode from given command", zap.Error(err))
		return "", err
	}
	return read(args)

}
