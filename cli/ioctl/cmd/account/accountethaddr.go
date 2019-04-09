// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// accountEthaddrCmd represents the account ethaddr command
var accountEthaddrCmd = &cobra.Command{
	Use:   "ethaddr [ALIAS|IOTEX_ADDRESS]",
	Short: "Derive ETH address from IoTeX address",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := accountEthaddr(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

func accountEthaddr(args []string) (string, error) {
	address, err := alias.Address(args[0])
	if err != nil {
		return "", err
	}
	ethAddr, err := util.IoAddrToEvmAddr(address)
	if err != nil {
		return "", err
	}
	return ethAddr.String(), nil
}
