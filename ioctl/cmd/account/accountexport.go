// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
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

// _accountExportCmd represents the account export command
var _accountExportCmd = &cobra.Command{
	Use:   config.TranslateInLang(_exportCmdUses, config.UILanguage),
	Short: config.TranslateInLang(_exportCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := accountExport(args[0])
		return output.PrintError(err)
	},
}

func accountExport(arg string) error {
	var (
		addr string
		err  error
	)
	if util.AliasIsHdwalletKey(arg) {
		addr = arg
	} else {
		addr, err = util.Address(arg)
		if err != nil {
			return output.NewError(output.AddressError, "failed to get address", err)
		}
	}
	prvKey, err := PrivateKeyFromSigner(addr, "")
	if err != nil {
		return output.NewError(output.KeystoreError, "failed to get private key from keystore", err)
	}
	output.PrintResult(prvKey.HexString())
	prvKey.Zero()
	return nil
}
