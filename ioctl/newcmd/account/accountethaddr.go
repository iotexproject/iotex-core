// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/pkg/util/addrutil"
)

// Multi-language support
var (
	ethaddrCmdShorts = map[config.Language]string{
		config.English: "Translate address between IOTX and ETH",
		config.Chinese: "在IOTX和ETH间转换地址",
	}
	ethaddrCmdUses = map[config.Language]string{
		config.English: "ethaddr (ALIAS|IOTEX_ADDRESS|ETH_ADDRESS)",
		config.Chinese: "ethaddr (别名|IOTEX_地址|ETH_地址)",
	}
)

// NewAccountEthAddr represents the account ethaddr command
func NewAccountEthAddr(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(ethaddrCmdUses)
	short, _ := client.SelectTranslation(ethaddrCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var ethAddress common.Address
			ioAddr, err := client.Address(args[0])
			if err != nil {
				if ok := common.IsHexAddress(args[0]); !ok {
					return errors.Wrap(err, "the input address is invalid")
				}
				ethAddress = common.HexToAddress(args[0])
				var ioAddress address.Address
				ioAddress, err = address.FromBytes(ethAddress.Bytes())
				if err != nil {
					return errors.Wrap(err, "failed to convert ETH address to IoTeX address")
				}
				ioAddr = ioAddress.String()
			} else {
				ethAddress, err = addrutil.IoAddrToEvmAddr(ioAddr)
				if err != nil {
					return err
				}
			}
			cmd.Println(fmt.Sprintf("%s - %s", ioAddr, ethAddress.String()))
			return nil
		},
	}
}
