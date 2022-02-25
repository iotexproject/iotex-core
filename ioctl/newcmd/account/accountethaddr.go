// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
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

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			var arg string
			if len(args) == 1 {
				arg = args[0]
			}

			var ethAddress common.Address
			ioAddr, err := util.Address(arg)
			if err != nil {
				if ok := common.IsHexAddress(arg); !ok {
					return output.NewError(output.AddressError, "", err)
				}
				ethAddress = common.HexToAddress(arg)
				ioAddress, err := address.FromBytes(ethAddress.Bytes())
				if err != nil {
					return output.NewError(output.AddressError, "failed to form IoTeX address from ETH address", nil)
				}
				ioAddr = ioAddress.String()
			} else {
				ethAddress, err = addrutil.IoAddrToEvmAddr(ioAddr)
				if err != nil {
					return output.NewError(output.AddressError, "", err)
				}
			}
			message := ethaddrMessage{IOAddr: ioAddr, EthAddr: ethAddress.String()}
			client.PrintInfo(message.String())
			return nil
		},
	}

	return cmd
}

type ethaddrMessage struct {
	IOAddr  string `json:"ioAddr"`
	EthAddr string `json:"ethAddr"`
}

func (m *ethaddrMessage) String() string {
	if !IsOutputFormat() {
		return fmt.Sprintf("%s - %s", m.IOAddr, m.EthAddr)
	}
	return output.FormatString(output.Result, m)
}
