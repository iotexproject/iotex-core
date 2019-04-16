// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// accountEthaddrCmd represents the account ethaddr command
var accountEthaddrCmd = &cobra.Command{
	Use:   "ethaddr [ALIAS|IOTEX_ADDRESS|ETH_ADDRESS]",
	Short: "Derive IoTeX or ETH address from ETH or IoTeX address",
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
	var ethAddress common.Address
	ioAddr, err := alias.Address(args[0])
	if err != nil {
		if ok := common.IsHexAddress(args[0]); !ok {
			return "", fmt.Errorf("invalid input")
		}
		ethAddress = common.HexToAddress(args[0])
		ioAddress, err := address.FromBytes(ethAddress.Bytes())
		if err != nil {
			return "", fmt.Errorf("failed to form IoTeX address from ETH address")
		}
		ioAddr = ioAddress.String()
	} else {
		ethAddress, err = util.IoAddrToEvmAddr(ioAddr)
		if err != nil {
			return "", err
		}
	}
	return ioAddr + " - " + ethAddress.String(), nil
}
