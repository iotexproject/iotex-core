// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package account

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	_exportCmdShorts = map[config.Language]string{
		config.English: "Export IoTeX private key from wallet",
		config.Chinese: "从钱包导出IoTeX的私钥",
	}
	_exportCmdUses = map[config.Language]string{
		config.English: "export (ALIAS|ADDRESS)",
		config.Chinese: "export (别名|地址)",
	}
)

// NewAccountExport represents the account export command
func NewAccountExport(client ioctl.Client) *cobra.Command {
	use, _ := client.SelectTranslation(_exportCmdUses)
	short, _ := client.SelectTranslation(_exportCmdShorts)

	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			arg := args[0]

			var (
				addr string
				err  error
			)
			if util.AliasIsHdwalletKey(arg) {
				addr = arg
			} else {
				addr, err = client.Address(arg)
				if err != nil {
					return errors.Wrap(err, "failed to get address")
				}
			}
			prvKey, err := PrivateKeyFromSigner(client, cmd, addr, "")
			if err != nil {
				return errors.Wrap(err, "failed to get private key from keystore")
			}
			cmd.Println(prvKey.HexString())
			prvKey.Zero()
			return nil
		},
	}
}
