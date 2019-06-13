// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

//Xrc20Cmd represent erc20 standard command-line
var Xrc20Cmd = &cobra.Command{
	Use:   "xrc20",
	Short: "Supporting ERC20 standard command-line from ioctl",
}

var xrc20ContractAddress string

func xrc20Contract() (address.Address, error) {
	return alias.IOAddress(xrc20ContractAddress)
}

func init() {
	Xrc20Cmd.AddCommand(xrc20TotalSupplyCmd)
	Xrc20Cmd.AddCommand(xrc20BalanceOfCmd)
	Xrc20Cmd.AddCommand(xrc20TransferCmd)
	Xrc20Cmd.AddCommand(xrc20TransferFromCmd)
	Xrc20Cmd.AddCommand(xrc20ApproveCmd)
	Xrc20Cmd.AddCommand(xrc20AllowanceCmd)
	Xrc20Cmd.PersistentFlags().StringVarP(&xrc20ContractAddress, "contract-address", "c", "",
		"set contract address")
	Xrc20Cmd.PersistentFlags().StringVar(&config.ReadConfig.Endpoint, "endpoint",
		config.ReadConfig.Endpoint, "set endpoint for once")
	Xrc20Cmd.PersistentFlags().BoolVar(&config.Insecure, "insecure", config.Insecure,
		"insecure connection for once (default false)")
	cobra.MarkFlagRequired(Xrc20Cmd.PersistentFlags(), "contract-address")
}
