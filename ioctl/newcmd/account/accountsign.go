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
	_signCmdShorts = map[config.Language]string{
		config.English: "Sign message with private key from wallet",
		config.Chinese: "用钱包中的私钥对信息签名",
	}
	_signCmdUses = map[config.Language]string{
		config.English: "sign MESSAGE [-s SIGNER]",
		config.Chinese: "sign 信息 [-s 签署人]",
	}
	_flagSignerUsages = map[config.Language]string{
		config.English: "choose a signing account",
		config.Chinese: "选择一个签名账户",
	}
)

// NewAccountSign represents the account sign command
func NewAccountSign(client ioctl.Client) *cobra.Command {
	var _signer string
	use, _ := client.SelectTranslation(_signCmdUses)
	short, _ := client.SelectTranslation(_signCmdShorts)
	usage, _ := client.SelectTranslation(_flagSignerUsages)

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			msg := args[0]

			var (
				addr string
				err  error
			)
			if util.AliasIsHdwalletKey(_signer) {
				addr = _signer
			} else {
				addr, err = client.Address(_signer)
				if err != nil {
					return errors.Wrap(err, "failed to get address")
				}
			}
			signedMessage, err := Sign(client, cmd, addr, "", msg)
			if err != nil {
				return errors.Wrap(err, "failed to sign message")
			}
			cmd.Println(signedMessage)
			return nil
		},
	}
	cmd.Flags().StringVarP(&_signer, "signer", "s", "", usage)
	return cmd
}
