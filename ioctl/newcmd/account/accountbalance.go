// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"math/big"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	ioAddress "github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
)

// Multi-language support
var (
	_balanceCmdUses = map[config.Language]string{
		config.English: "balance [ALIAS|ADDRESS]",
		config.Chinese: "balance [别名|地址]",
	}
	_balanceCmdShorts = map[config.Language]string{
		config.English: "Get balance of an account",
		config.Chinese: "查询账号余额",
	}
)

// NewAccountBalance represents the account balance command
func NewAccountBalance(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_balanceCmdUses)
	short, _ := client.SelectTranslation(_balanceCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			addr := ""
			if len(args) == 1 {
				addr = args[0]
			}
			if addr != ioAddress.StakingBucketPoolAddr && addr != ioAddress.RewardingPoolAddr {
				var err error
				addr, err = client.AddressWithDefaultIfNotExist(addr)
				if err != nil {
					return errors.Wrap(err, "failed to get address")
				}
			}

			accountMeta, err := Meta(client, addr)
			if err != nil {
				return errors.Wrap(err, "failed to get account meta")
			}
			balance, ok := new(big.Int).SetString(accountMeta.Balance, 10)
			if !ok {
				return errors.Wrap(err, "failed to convert account balance")
			}
			cmd.Println(fmt.Sprintf("%s: %s IOTX", addr, balance))
			return nil
		},
	}
}
